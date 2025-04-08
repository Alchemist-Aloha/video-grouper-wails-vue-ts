# Video Grouper

A desktop application built with Wails (Go backend) and Vue.js + TypeScript that helps organize your video files by grouping them into folders.

## Features

- Select a base video directory to scan for video files
- Display videos with automatically generated thumbnails
- Select multiple videos to organize
- Move selected videos into a new folder named after the first video
- Real-time status updates and error reporting

## Technologies Used

- [Wails v2](https://wails.io/) - Go framework for building desktop applications
- Go - Backend implementation for file system operations
- Vue.js 3 - Frontend framework
- TypeScript - Type safety for JavaScript
- FFmpeg - Used for generating video thumbnails

## Prerequisites

- Go 1.23 or later
- Node.js and npm
- Wails CLI
- FFmpeg installed and available in PATH (required for thumbnail generation)

## Installation

1. Clone this repository:
   ```
   git clone https://github.com/Alchemist-Aloha/video-grouper-wails-vue-ts.git
   ```

2. Install frontend dependencies:
   ```
   cd video-grouper-wails-vue-ts/frontend
   ```
   ```
   npm install
   ```

3. Build the application:
   ```
   cd ..
   ```
   ```
   wails build
   ```

## Development

To start the application in development mode:

```
wails dev
```

This will start both the Go backend server and the Vue.js development server with hot reloading.

## Usage

1. Launch the application
2. Click "Select Base Video Directory" to choose the directory containing your videos
3. The application will scan for video files and generate thumbnails
4. Select the videos you want to group together
5. Click "Move Selected Videos" to move them to a new folder
   - The new folder will be created using the name of the first selected video

## How It Works

### Backend (Go)

- `SelectDirectory()` - Opens a directory selection dialog and scans for video files
- `GenerateThumbnail()` - Uses FFmpeg to generate thumbnail images from video files
- `MoveVideos()` - Moves selected videos to a new directory

### Frontend (Vue.js + TypeScript)

- Interactive user interface for selecting and managing videos
- Real-time updates on operations through Wails event system
- Thumbnail display for visual identification of videos

## Building for Production

To build the application for production:

```
wails build -platform windows/darwin/linux
```

The compiled binary will be available in the `build/bin` directory.

## Important Notes

- The application moves original video files, so ensure you have backups if needed
- FFmpeg must be installed and available in your system PATH for thumbnail generation
- Currently optimized for .m4v files, but can be extended for other formats

## License

MIT License