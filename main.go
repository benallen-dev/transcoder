package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"transcoder/config"
	"transcoder/logger"

	"github.com/goforj/godump"
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
	logger.Info("Moving input that saw an error to problem dir", "file", f)
	return os.Rename(ogLoc, newLoc)
}

func moveFileToDoneDir(conf *config.Config, f string) error {
	ogLoc := filepath.Join(conf.Dirs.Watch, f)
	newLoc := filepath.Join(conf.Dirs.Done, f)
	logger.Info("Moving processed file to done", "file", f)
	return os.Rename(ogLoc, newLoc)
}

func encodeFile(conf *config.Config, f string) error {
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

	cmd := exec.Command("ffmpeg",
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

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		logger.Error("Error running encoding command, removing output file, ", "file", targetFile)
		os.Remove(targetFile)
		return NewCommandExecutionError(f, cmd.String(), err)
	}

	moveFileToDoneDir(conf, f)

	return nil
}

func main() {
	conf, err := config.Load()
	if err != nil {
		logger.Fatal(err)
	}
	godump.Dump(conf)

	// Until SIGTERM or SIGINT
	for true {
		// Get the first file in the watch dir
		files, err := os.ReadDir(conf.Dirs.Watch)
		if err != nil {
			logger.Fatal(err)
		}

		if len(files) < 1 { // If there are none, take a nap
			logger.Info("No files in watch dir")
			time.Sleep(conf.NapTime)
			continue
		}

		// Assemble the input filename
		f := files[0].Name()
		err = encodeFile(conf, f)
		if err != nil {
			switch err.(type) {
			case FileNotReadyError:
				// Place it at the back of the queue once we have
				// one, but for now just wait and try again
				time.Sleep(conf.NapTime)
				continue
			default:
				logger.Error(err.Error())
				moveFileToProblemDir(conf, f)
				continue
			}
		}

		moveFileToDoneDir(conf, f)
	}
}
