package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct (Unchanged)
type App struct {
	ctx context.Context
}

// NewApp (Unchanged)
func NewApp() *App {
	return &App{}
}

// startup (Unchanged)
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	runtime.LogInfo(a.ctx, "Go Backend Started.")
}

// --- Bound Go Functions ---

// SelectDirectory scans the selected directory for video files and returns their paths.
func (a *App) SelectDirectory() ([]string, error) {
	runtime.LogInfo(a.ctx, "SelectDirectory called from frontend.")
	selectedDir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Base Video Directory",
	})
	if err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Error selecting directory: %v", err))
		return nil, err
	}
	if selectedDir == "" {
		runtime.LogInfo(a.ctx, "Directory selection cancelled.")
		return nil, nil
	}

	runtime.LogInfo(a.ctx, fmt.Sprintf("Directory selected: %s", selectedDir))

	// Scan the directory for video files
	var videoFiles []string
	err = filepath.Walk(selectedDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Check if the file is a video (basic check based on extension)
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".m4v") {
			videoFiles = append(videoFiles, path)
		}
		return nil
	})
	if err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Error scanning directory: %v", err))
		return nil, err
	}

	runtime.LogInfo(a.ctx, fmt.Sprintf("Found %d video files in directory.", len(videoFiles)))
	return videoFiles, nil
}

// MoveVideos takes a list of absolute video file paths and moves them
// into a new directory named after the first video.
func (a *App) MoveVideos(absoluteFilePaths []string) error {
	runtime.LogInfo(a.ctx, fmt.Sprintf("MoveVideos called with %d files.", len(absoluteFilePaths)))
	runtime.EventsEmit(a.ctx, "move-status", fmt.Sprintf("Received request to move %d files.", len(absoluteFilePaths)))

	if len(absoluteFilePaths) == 0 {
		err := fmt.Errorf("no video files provided to move")
		runtime.LogError(a.ctx, err.Error())
		runtime.EventsEmit(a.ctx, "move-error", err.Error())
		return err
	}

	// --- Determine Output Directory Path ---
	firstFilePath := absoluteFilePaths[0]
	// baseName := filepath.Base(firstFilePath)
	// ext := filepath.Ext(baseName)
	// outputFolderName := strings.TrimSuffix(baseName, ext) // Folder name is filename without extension

	// // Use the parent directory of the first video file as the base directory
	// parentDir := filepath.Dir(firstFilePath)
	// runtime.LogInfo(a.ctx, fmt.Sprintf("parentDir: %s", parentDir))
	// runtime.LogInfo(a.ctx, fmt.Sprintf("outputFolderName: %s", outputFolderName))
	// outputDir := filepath.Join(parentDir, outputFolderName) // Use filepath.Join for correct path construction
	outputDir := strings.Join(strings.Split(firstFilePath, "."), "_")
	runtime.LogInfo(a.ctx, fmt.Sprintf("Target output directory for moved files: %s", outputDir))
	runtime.EventsEmit(a.ctx, "move-status", fmt.Sprintf("Target directory: %s", outputDir))

	// --- Create Output Directory ---
	err := os.Mkdir(outputDir, os.ModePerm) // 0755 permission
	if err != nil {
		errMsg := fmt.Sprintf("failed to create output directory '%s': %v", outputDir, err)
		runtime.LogError(a.ctx, errMsg)
		runtime.EventsEmit(a.ctx, "move-error", errMsg)
		return fmt.Errorf(errMsg)
	}
	runtime.LogInfo(a.ctx, "Output directory created or already exists.")
	runtime.EventsEmit(a.ctx, "move-status", "Output directory created.")

	// --- Move Files ---
	movedCount := 0
	runtime.EventsEmit(a.ctx, "move-status", "Starting file move process...")
	for _, originalPath := range absoluteFilePaths {
		fileName := filepath.Base(originalPath)
		newPath := filepath.Join(outputDir, fileName)

		runtime.LogInfo(a.ctx, fmt.Sprintf("Attempting to move '%s' to '%s'", originalPath, newPath))
		runtime.EventsEmit(a.ctx, "move-status", fmt.Sprintf("Moving %s...", fileName))

		// Use os.Rename to move the file.
		err := os.Rename(originalPath, newPath)
		if err != nil {
			// Attempt to provide more context on error
			_, statErr := os.Stat(originalPath)
			if os.IsNotExist(statErr) {
				errMsg := fmt.Sprintf("Failed to move file '%s': Source file not found.", fileName)
				runtime.LogError(a.ctx, errMsg)
				runtime.EventsEmit(a.ctx, "move-error", errMsg)
				return fmt.Errorf(errMsg) // Stop on critical error
			}

			// Generic rename error
			errMsg := fmt.Sprintf("Failed to move file '%s' to '%s': %v", fileName, newPath, err)
			runtime.LogError(a.ctx, errMsg)
			runtime.EventsEmit(a.ctx, "move-error", errMsg+". Might be cross-drive issue or permissions.")
			return fmt.Errorf(errMsg)
		}
		runtime.LogInfo(a.ctx, fmt.Sprintf("Successfully moved %s", fileName))
		movedCount++
	}

	successMsg := fmt.Sprintf("Successfully moved %d files to %s", movedCount, outputDir)
	runtime.LogInfo(a.ctx, successMsg)
	runtime.EventsEmit(a.ctx, "move-complete", successMsg) // Emit final success message

	return nil // Success
}

// GenerateThumbnail generates a thumbnail for a given video file and returns it as a Data URL.
func (a *App) GenerateThumbnail(videoPath string) (string, error) {
	runtime.LogInfo(a.ctx, fmt.Sprintf("Generating thumbnail Data URL for: %s", videoPath))

	// ffmpeg command arguments:
	// -i videoPath : Input video file
	// -ss 00:00:01 : Seek to the 1-second mark (adjust if needed)
	// -vframes 1  : Extract exactly one video frame
	// -f image2pipe : Force the output format to be suitable for piping (image sequence)
	// -c:v mjpeg    : Set the video codec for the output image to MJPEG (JPEG)
	// -           : Output to stdout instead of a file
	cmd := exec.Command("ffmpeg", "-i", videoPath, "-ss", "00:00:01", "-vframes", "1", "-f", "image2pipe", "-c:v", "mjpeg", "-")

	// Create buffers to capture stdout and stderr
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb // Capture standard output
	cmd.Stderr = &errb // Capture standard error

	// Run the command
	err := cmd.Run()
	if err != nil {
		// If ffmpeg fails, log the error and stderr content for diagnostics
		errMsg := fmt.Sprintf("ffmpeg execution failed for %s: %v. Stderr: %s", videoPath, err, errb.String())
		runtime.LogError(a.ctx, errMsg)
		return "", fmt.Errorf(errMsg) // Return an empty string and the error
	}

	// Get the raw image bytes from the stdout buffer
	imageBytes := outb.Bytes()

	// Check if ffmpeg actually produced any output
	if len(imageBytes) == 0 {
		errMsg := fmt.Sprintf("ffmpeg produced no thumbnail data for %s. Stderr: %s", videoPath, errb.String())
		// Log as warning or error based on whether stderr had content
		if errb.Len() > 0 {
			runtime.LogWarning(a.ctx, errMsg) // May be warnings in stderr even on success
		} else {
			runtime.LogError(a.ctx, errMsg) // No output and no stderr likely means a problem
		}
		// Return error as we expect image data
		return "", fmt.Errorf("ffmpeg produced no thumbnail data for video: %s", videoPath)
	}

	// Encode the raw image bytes to a Base64 string
	encodedString := base64.StdEncoding.EncodeToString(imageBytes)

	// Format the Base64 string as a JPEG Data URL
	// The MIME type "image/jpeg" matches the "-c:v mjpeg" ffmpeg argument.
	// If you change the codec (e.g., to png), update the MIME type accordingly.
	dataURL := fmt.Sprintf("data:image/jpeg;base64,%s", encodedString)

	runtime.LogInfo(a.ctx, fmt.Sprintf("Successfully generated thumbnail Data URL for: %s (Data URL length: %d)", videoPath, len(dataURL)))

	// Return the Data URL string and nil error
	return dataURL, nil
}
