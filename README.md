# Transcoder

Watches files in a directory, then drags them kicking and screaming through ffmpeg. It's really basic, but I had a lot of files to transcode.

## Depends on

- lsof
- ffmpeg

Keep in mind the program doesn't actually check if these are available. I'm not even sure if it crashes or just has weird behaviour if they're missing.

## Configuration

Configuration is done in config.toml.

```toml
# NapTime is how many seconds to wait before checking the watch dir again if it's empty or a file isn't ready for opening
NapTime = "5s"

[Dirs]
Watch = "the directory to watch"
Output = "directory where output files will be written"
Done = "directory where source files will be moved after being processed"
Problem = "directory where source files will be moved if an error occurs during transcoding"

[Output]
MaxWidth = 1280
MaxHeight = 720
MaxBitrate = 4 # Mbit/s
Crf = 26 # 0-51, default 28. Higher numbers mean more compression.
```
