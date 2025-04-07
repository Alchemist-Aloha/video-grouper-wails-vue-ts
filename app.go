package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"bytes"
	"encoding/base64"
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

// SelectDirectory (Unchanged)
func (a *App) SelectDirectory() (string, error) {
	runtime.LogInfo(a.ctx, "SelectDirectory called from frontend.")
	selectedDir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Base Video Directory",
	})
	// ... (rest of the function is the same)
	if err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Error selecting directory: %v", err))
		return "", err
	}
	if selectedDir == "" {
		runtime.LogInfo(a.ctx, "Directory selection cancelled.")
	} else {
		runtime.LogInfo(a.ctx, fmt.Sprintf("Directory selected: %s", selectedDir))
	}
	return selectedDir, nil
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
	baseName := filepath.Base(firstFilePath)
	ext := filepath.Ext(baseName)
	outputFolderName := strings.TrimSuffix(baseName, ext) // Folder name is filename without extension

	// Output folder will be created *beside* the parent dir of the first video's original base directory
	// Assumes firstFilePath is like /path/to/baseDir/maybe/subdir/video.mp4
	// We need the parent of baseDir. This depends on how baseDir was selected and paths constructed.
	// Let's assume the paths given are like /path/to/BASE_DIR/video.mp4 or /path/to/BASE_DIR/subdir/video.mp4
	// We need the parent of BASE_DIR.

	// Find the common base directory implied by the first path.
	// This assumes SelectDirectory gave us the intended 'base'. We should perhaps pass it explicitly.
	// Let's stick to the previous logic: create folder beside parent of the first file's immediate dir.
	// This might not be exactly the parent of the 'selected base directory' if videos are in subdirs.
	// A more robust approach would be to pass baseDirectory from JS to Go.
	// Sticking to original logic for now: Parent of the first file's directory.
	parentDir := filepath.Dir(filepath.Dir(firstFilePath)) // Go up two levels from the file path
	outputDir := filepath.Join(parentDir, outputFolderName)

	runtime.LogInfo(a.ctx, fmt.Sprintf("Target output directory for moved files: %s", outputDir))
	runtime.EventsEmit(a.ctx, "move-status", fmt.Sprintf("Target directory: %s", outputDir))

	// --- Create Output Directory ---
	err := os.MkdirAll(outputDir, os.ModePerm) // 0755 permission
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
		// Note: This usually only works reliably on the same filesystem/volume.
		// For cross-volume moves, a copy + delete approach is needed.
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
			// Decide whether to stop or continue. Let's stop on first error for simplicity.
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

// GenerateThumbnail generates a thumbnail for a given video file.
// func (a *App) GenerateThumbnail(videoPath string) (string, error) {
// 	runtime.LogInfo(a.ctx, fmt.Sprintf("Generating thumbnail for: %s", videoPath))
// 	thumbnailPath := filepath+"_thumbnail.jpg" // Change this to your desired thumbnail path
// 	cmd := exec.Command("ffmpeg", "-i", videoPath, "-ss", "00:00:01", "-vframes", "1", thumbnailPath)

// 	err := cmd.Run()
// 	if err != nil {
// 		errMsg := fmt.Sprintf("Failed to generate thumbnail for %s: %v", videoPath, err)
// 		runtime.LogError(a.ctx, errMsg)
// 		return "", fmt.Errorf(errMsg)
// 	}

// 	runtime.LogInfo(a.ctx, fmt.Sprintf("Thumbnail generated at: %s", thumbnailPath))
// 	return thumbnailPath, nil
// }

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