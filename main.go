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

func moveFileToProblemDir(conf *config.Config, file string) error {
	ogLoc := filepath.Join(conf.Dirs.Watch, file)
	newLoc := filepath.Join(conf.Dirs.Problem, file)
	logger.Info("Moving input that saw an error to problem dir", "file", file)
	return os.Rename(ogLoc, newLoc)
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
		loc := filepath.Join(conf.Dirs.Watch, f)

		// Check if the file is open in another process
		isOpen, err := isFileOpen(loc)
		if err != nil {
			logger.Error("Error checking if file is open", "file", f, "error", err)
			moveFileToProblemDir(conf, f)
			continue
		}

		if isOpen {
			logger.Info("File is not ready, waiting", "file", f)
			time.Sleep(conf.NapTime)
			continue
		}

		// Build the ffmpeg command
		var width = strconv.Itoa(conf.Output.MaxWidth)
		var height = strconv.Itoa(conf.Output.MaxHeight)
		var bitrate = strconv.Itoa(conf.Output.MaxBitrate)
		var bufsize = strconv.Itoa(2 * conf.Output.MaxBitrate)
		var targetFile = filepath.Join(conf.Dirs.Output, f+".mkv")

		cmd := exec.Command("ffmpeg",
			// "-hwaccel", "cuda",
			"-i", loc,
			"-vf", fmt.Sprintf("scale='min(%s,iw)':'min(%s,ih)':force_original_aspect_ratio=decrease", width, height),
			"-b:v", bitrate+"M",
			"-maxrate", bitrate+"M",
			"-bufsize", bufsize+"M",
			"-map", "0:v:0",
			"-map", "0:m:language:eng",
			"-map", "0:a:0",
			"-ac", "2",
			"-c:v", "libx265",
			"-preset", "fast",
			// "-tag", "hvc1",
			"-crf", "27",
			"-c:a", "aac",
			"-b:a", "192k",
			targetFile,
		)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err = cmd.Run()
		if err != nil {
			logger.Error("Failed to run command")
			logger.Error(cmd.String())
			logger.Error(err.Error())

			moveFileToProblemDir(conf, f)

			logger.Error("Removing output file", "file", targetFile)
			os.Remove(targetFile)
			continue
		}

		// Move the source file from Watch dir to Done dir
		newLoc := filepath.Join(conf.Dirs.Done, f)
		logger.Info("Moving processed file to done", "file", f)
		os.Rename(loc, newLoc)
	}

}
