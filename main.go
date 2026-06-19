package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"transcoder/config"
	"transcoder/logger"
)

func isFileOpen(path string) (bool, error) {
	out, err := exec.Command("lsof", path).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil // no output from lsof = not open
		}
		return false, err
	}

	return strings.TrimSpace(string(out)) != "", nil
}

func moveFileToProblemDir(conf *config.Config, f string) error {
	ogLoc := filepath.Join(conf.Dirs.Watch, f)
	newLoc := filepath.Join(conf.Dirs.Problem, f)
	return os.Rename(ogLoc, newLoc)
}

func moveFileToDoneDir(conf *config.Config, f string) error {
	ogLoc := filepath.Join(conf.Dirs.Watch, f)
	newLoc := filepath.Join(conf.Dirs.Done, f)
	// logger.Info("Moving processed file to done", "file", f)
	return os.Rename(ogLoc, newLoc)
}

func encodeFile(ctx context.Context, conf *config.Config, worker int, f string) error {
	logPrefix := fmt.Sprintf("[ \033[33mworker-%02d \033[35mencodeFile\033[0m ] ", worker)
	loc := filepath.Join(conf.Dirs.Watch, f)

	// Check if the file is open in another process
	isOpen, err := isFileOpen(loc)
	if err != nil {
		return err
	}
	if isOpen {
		return NewFileNotReadyError(loc)
	}

	// Build the ffmpeg command
	var width = strconv.Itoa(conf.Output.MaxWidth)
	var height = strconv.Itoa(conf.Output.MaxHeight)
	var bitrate = strconv.Itoa(conf.Output.MaxBitrate)
	var bufsize = strconv.Itoa(2 * conf.Output.MaxBitrate)
	var crf = strconv.Itoa(conf.Output.Crf)
	var targetFile = filepath.Join(conf.Dirs.Output, f+".mp4")

	logger.Info(fmt.Sprintf("%sStart transcoding: %s", logPrefix, f))

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y", // yes to prompts
		"-i", loc,
		"-vf", fmt.Sprintf("scale='min(%s,iw)':'min(%s,ih)':force_original_aspect_ratio=decrease,scale=trunc(iw/2)*2:trunc(ih/2)*2", width, height),

		"-b:v", bitrate+"M",
		"-maxrate", bitrate+"M",
		"-bufsize", bufsize+"M",
		"-map", "0:v:0",
		"-map", "0:m:language:eng",
		"-map", "0:a:0",
		"-map", "-0:s", // strip subs
		"-ac", "2",

		"-c:v", "libx265",
		"-tag:v", "hvc1",
		"-crf", crf,
		"-x265-params", "keyint=60:min-keyint=60:no-scenecut=1",
		"-movflags", "+faststart",

		"-preset", "fast",
		"-c:a", "aac",
		"-b:a", "192k",
		targetFile,
	)

	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return NewCommandCancellationError(err)
		}
		// Killed by OS
		if exitErr, ok := err.(*exec.ExitError); ok {
			if !exitErr.ProcessState.Exited() {
				logger.Debug(logPrefix+"Process killed by signal")
			}
			return NewCommandCancellationError(err)
		}

		logger.Error(logPrefix+"Error running encoding command, removing output file, ", "file", targetFile)
		os.Remove(targetFile)
		return NewCommandExecutionError(f, cmd.String(), err)
	}

	logger.Info(fmt.Sprintf("%sTranscode complete: %s", logPrefix, f))
	moveFileToDoneDir(conf, f)

	return nil
}

func worker(id int, conf *config.Config, files <-chan string, ctx context.Context) {
	logPrefix := fmt.Sprintf("[ \033[33mworker-%02d\033[0m ] ", id)

	logger.Info(logPrefix+"Started worker")

	for {
		select {
		case <-ctx.Done():
			logger.Info(logPrefix + "Shutting down")
			return
		case f, ok := <-files:
			if !ok {
				logger.Info(logPrefix + "Channel closed, shutting down")
				return
			}

			logger.Debug(logPrefix+"Processing file", "file", f)
			err := encodeFile(ctx, conf, id, f)
			logger.Debug(logPrefix+"Finished processing file", "file", f)
			if err != nil {
				switch err.(type) {
				case *FileNotReadyError:
					// Place it at the back of the queue once we have
					// one, but for now just wait and try again
					time.Sleep(conf.NapTime)
					continue
				case *CommandCancellationError:
					// Do nothing, we're just shutting down
					logger.Debug(logPrefix+"Shutting down worker, context canceled")
					return
				default:
					logger.Error(err.Error())
					logger.Info(logPrefix+"Moving input that saw an error to problem dir", "file", f)
					moveFileToProblemDir(conf, f)
					continue
				}
			}

			moveFileToDoneDir(conf, f)
		}
	}
}

func watchDir(conf *config.Config, fileChan chan<- string, ctx context.Context) {
	logPrefix := "[ \033[36mwatchDir\033[0m ] "
	seen := make(map[string]bool)
	ticker := time.NewTicker(conf.NapTime)
	defer ticker.Stop()

	logger.Info(logPrefix+"Started watchDir")

	for {
		select {
		case <-ctx.Done():
			logger.Info(logPrefix+"Shutting down")
			close(fileChan)
			return
		case <-ticker.C:
			files, err := os.ReadDir(conf.Dirs.Watch)
			if err != nil {
				logger.Error(logPrefix+"Error reading watch dir", "error", err)
				continue
			}

			if len(files) < 1 {
				logger.Debug(logPrefix+"No files found in watch dir")
			}

			for _, file := range files {
				name := file.Name()
				// Check make sure context isn't canceled
				if !seen[name] {
					seen[name] = true
					logger.Info(fmt.Sprintf("%sAdding %s", logPrefix, name))
					select {
					case <-ctx.Done():
						return
					case fileChan <- name:
					default:
						// If channel full or closed, exit
						logger.Warn(logPrefix+"could not send, but also not ctx.Done()")
						return
					}
				}
			}
		}
	}
}

func main() {
	conf, err := config.Load()
	if err != nil {
		logger.Fatal(err)
	}

	var wg sync.WaitGroup

	// numWorkers := max(runtime.NumCPU() / 2, 1)
	numWorkers := 3
	ctx, cancel := context.WithCancel(context.Background())

	// Handle program signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	fileChan := make(chan string, 1000) // I'm never gonna need 1000 files... right?

	go func() {
		logger.Debug("Watching for OS signals")
		sig := <-sigChan

		fmt.Println() // clear console line
		logger.Info("Recieved signal", "signal", sig)
		cancel()
	}()

	// Build worker pool
	logger.Info("Starting worker pool", "workers", numWorkers)

	for i := range numWorkers {
		// logger.Info(fmt.Sprintf("Starting worker %d", i))
		wg.Go(func() { worker(i, conf, fileChan, ctx) })
	}

	// Watch dir for files
	wg.Go(func() { watchDir(conf, fileChan, ctx) })

	// Wait until all goroutines exit
	wg.Wait()
}
