<template>
  <div class="video-manager">
    <h2>Video Mover Tool (Wails + Go Backend)</h2>

    <button @click="selectBaseDirectory">Select Base Video Directory</button>
    <p v-if="baseDirectory">
      Base Directory: <code>{{ baseDirectory }}</code>
    </p>
    <p v-else class="warning">
      Please select the base directory containing your videos first.
    </p>

    <input
      type="file"
      webkitdirectory
      multiple
      @change="handleFolderSelect"
      ref="folderInput"
      style="display: none;"
      accept="video/*"
    />
    <button @click="triggerFolderSelect" :disabled="!baseDirectory">
      Load Videos from Selected Directory
    </button>

    <div v-if="isLoading" class="loading">Generating thumbnails...</div>

    <div v-if="videos.length > 0" class="video-list-container">
      <h3>Loaded Videos:</h3>
      <ul class="video-list">
         <li v-for="video in videos" :key="video.id" class="video-item">
          <input
            type="checkbox"
            :id="'video-' + video.id"
            v-model="video.selected"
          />
          <label :for="'video-' + video.id">
            <img
              v-if="video.thumbnail"
              :src="video.thumbnail"
              alt="Video thumbnail"
              class="thumbnail"
            />
            <div v-else class="thumbnail placeholder">No thumbnail</div>
            <span class="filename">
                {{ video.file.webkitRelativePath || video.file.name }}
            </span>
            <span v-if="video.error" class="error-tag">Error</span>
          </label>
        </li>
      </ul>

      <button
        @click="moveSelectedVideos"
        :disabled="selectedVideos.length === 0 || !baseDirectory || isMoving"
      >
        {{ isMoving ? 'Moving...' : 'Move Selected Videos' }}
      </button>
      <p v-if="selectedVideos.length > 0">
        Selected {{ selectedVideos.length }} video(s).
      </p>
       <p v-if="moveError" class="error-message">
        Move Error: {{ moveError }}
      </p>
      <p v-if="moveSuccessMessage" class="success-message">
        {{ moveSuccessMessage }}
      </p>
    </div>

    <div v-if="logMessages.length > 0" class="log-output">
        <h3>Log / Status:</h3>
        <pre>{{ logMessages.join('\n') }}</pre>
    </div>

    <div class="explanation">
      <h4>Important Notes:</h4>
      <ul>
        <li>Select the Base Directory first. Then Load videos.</li>
        <li>Thumbnails are generated in the browser.</li>
        <li>Clicking "Move" will move the selected original video files into a new subfolder.</li>
        <li>The new folder (named after the first selected video) will be created inside the *parent* of your selected Base Directory.</li>
        <li>**Warning:** This action moves the original files. Ensure you have backups if needed.</li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue';
// *** MODIFIED: Import MoveVideos instead of MergeVideos ***
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime';
import { SelectDirectory, MoveVideos } from '../wailsjs/go/main/App';

// --- Define Interface for Video Objects ---
interface VideoItem {
  id: number;
  file: File; // Browser's built-in File type
  thumbnail: string | null; // Data URL string or null
  selected: boolean;
  error: string | null; // Error message or null
}

// --- State ---
const folderInput = ref<HTMLInputElement | null>(null);
const videos = reactive<VideoItem[]>([]);
const isLoading = ref<boolean>(false);
const logMessages = ref<string[]>([]);
const baseDirectory = ref<string>('');
// *** MODIFIED: State variables for moving status ***
const isMoving = ref<boolean>(false);
const moveError = ref<string>('');
const moveSuccessMessage = ref<string>('');

let videoIdCounter: number = 0;

// --- Computed Properties (Unchanged) ---
const selectedVideos = computed<VideoItem[]>(() => {
  return videos.filter(video => video.selected);
});

// --- Wails Event Handling ---
onMounted(() => {
    // *** MODIFIED: Event names ***
    EventsOn("move-status", (status: string) => {
        log(`[Go Backend] ${status}`);
    });
    EventsOn("move-complete", (successMsg: string) => {
        log(`[Go Backend] ${successMsg}`);
        moveSuccessMessage.value = successMsg;
        isMoving.value = false;
        moveError.value = '';
        // Clear the list as the files have been moved
        videos.splice(0, videos.length);
    });
    EventsOn("move-error", (errorMsg: string) => {
        log(`[Go Backend] Move Error: ${errorMsg}`);
        moveError.value = errorMsg;
        isMoving.value = false;
        moveSuccessMessage.value = '';
    });
});

onBeforeUnmount(() => {
    // *** MODIFIED: Event names ***
    EventsOff("move-status");
    EventsOff("move-complete");
    EventsOff("move-error");
});


// --- Methods ---

function log(message: string): void {
  console.log(message);
  const timestamp = new Date().toLocaleTimeString();
  logMessages.value.push(`[${timestamp}] ${message}`);
  if (logMessages.value.length > 100) { // Keep log size reasonable
      logMessages.value.shift();
  }
}

// 1. NEW: Call Go to select the base directory
async function selectBaseDirectory() {
  log("Requesting base directory selection from Go...");
  try {
    const selectedDir = await SelectDirectory(); // Call bound Go function
    if (selectedDir) {
      baseDirectory.value = selectedDir;
      log(`Base directory set: ${selectedDir}`);
      // Clear previous videos if base directory changes
      videos.splice(0, videos.length);
      logMessages.value = []; // Clear log
    } else {
      log("Directory selection cancelled or failed.");
    }
  } catch (err: any) {
    log(`Error selecting directory: ${err}`);
    alert(`Error selecting directory: ${err}`);
  }
}

// 2. Trigger the hidden HTML file input
function triggerFolderSelect() {
  if (!baseDirectory.value) {
    alert("Please select the base directory first!");
    return;
  }
  logMessages.value = []; // Clear log on new load
  folderInput.value?.click(); // Trigger hidden input
}

// 3. Handle HTML input file selection (mostly for File objects)
async function handleFolderSelect(event: Event): Promise<void> {
  const target = event.target as HTMLInputElement;
  const files = target.files;

  if (!files || files.length === 0) {
    log("No files selected in the input.");
    return;
  }
   if (!baseDirectory.value) {
    log("Error: Base directory not set before loading files.");
    alert("Error: Base directory not set. Please select it first.");
    return;
  }

  log(`HTML input provided ${files.length} items. Filtering for videos...`);
  isLoading.value = true;
  videos.splice(0, videos.length); // Clear previous videos

  const videoFiles: File[] = Array.from(files).filter(file => file.type.startsWith('video/'));

  if (videoFiles.length === 0) {
      log("No video files found matching the input files.");
      isLoading.value = false;
      if (target) target.value = '';
      return;
  }

  log(`Found ${videoFiles.length} video files from input. Generating thumbnails...`);

  // Create initial video objects
  videoFiles.forEach(file => {
    // IMPORTANT: Check if webkitRelativePath exists and seems valid relative to base
    // This is heuristic - might need adjustment based on testing
    const relativePath = file.webkitRelativePath || file.name;
    log(`Processing file: ${relativePath}`);

    videos.push({
      id: videoIdCounter++,
      file: file, // Keep the File object for thumbnail generation
      thumbnail: null,
      selected: false,
      error: null,
    });
  });

  // Generate thumbnails (client-side, unchanged)
  const thumbnailPromises: Promise<void>[] = videos.map(video => generateThumbnail(video));
  await Promise.allSettled(thumbnailPromises);

  isLoading.value = false;
  log("Thumbnail generation complete.");
  if (target) target.value = ''; // Reset input
}

// 4. Generate Thumbnails (Client-side - Unchanged from TypeScript version)
function generateThumbnail(videoObject: VideoItem): Promise<void> {
    // ... (Keep the exact same implementation as the previous TypeScript answer)
    // Ensure it handles errors and updates videoObject.thumbnail / videoObject.error
  return new Promise<void>((resolve, reject) => { // Specify Promise<void>
    const videoFile: File = videoObject.file;
    let videoUrl: string | null = null; // Keep track to revoke later
    let timeoutId: number | undefined = undefined; // For setTimeout handle

    try {
        videoUrl = URL.createObjectURL(videoFile);
    } catch (error) {
        videoObject.error = `Failed to create Object URL: ${(error as Error).message}`;
        log(`Error for ${videoFile.name}: ${videoObject.error}`);
        reject(new Error(videoObject.error));
        return;
    }

    const videoElement = document.createElement('video');
    const canvasElement = document.createElement('canvas');
    const context = canvasElement.getContext('2d');
    const targetTime: number = 1.0;

    videoElement.preload = 'metadata';

    const cleanup = () => {
        if (timeoutId !== undefined) clearTimeout(timeoutId);
        if (videoUrl) URL.revokeObjectURL(videoUrl);
        videoUrl = null;
        videoElement.onloadedmetadata = null;
        videoElement.onseeked = null;
        videoElement.onerror = null;
        videoElement.src = '';
        videoElement.removeAttribute('src');
        videoElement.load();
    };

    videoElement.onloadedmetadata = () => {
      const aspectRatio = videoElement.videoWidth / videoElement.videoHeight;
      canvasElement.width = 160;
      canvasElement.height = canvasElement.width / aspectRatio;
      const seekTime = Math.min(targetTime, videoElement.duration || targetTime);
      videoElement.currentTime = seekTime;
    };

    videoElement.onseeked = () => {
      if (!context) {
        videoObject.error = "Failed to get 2D canvas context.";
        log(`Error generating thumbnail for ${videoFile.name}: ${videoObject.error}`);
        cleanup();
        reject(new Error(videoObject.error));
        return;
      }
      try {
        context.drawImage(videoElement, 0, 0, canvasElement.width, canvasElement.height);
        videoObject.thumbnail = canvasElement.toDataURL('image/jpeg', 0.7);
        cleanup();
        resolve();
      } catch (drawError) {
          videoObject.error = `Failed to draw image on canvas: ${(drawError as Error).message}`;
          log(`Error generating thumbnail for ${videoFile.name}: ${videoObject.error}`);
          cleanup();
          reject(new Error(videoObject.error));
      }
    };

    videoElement.onerror = (e: Event | string) => {
      const errorMsg = (typeof e === 'string') ? e : (videoElement.error?.message || 'Unknown video loading error');
      videoObject.error = `Video load/seek failed: ${errorMsg}`;
      log(`Error generating thumbnail for ${videoFile.name}: ${videoObject.error}`);
      cleanup();
      reject(new Error(videoObject.error));
    };

    timeoutId = window.setTimeout(() => {
        if (!videoObject.thumbnail && !videoObject.error) {
            videoObject.error = "Thumbnail generation timed out.";
            log(`Error generating thumbnail for ${videoFile.name}: Timeout.`);
            cleanup();
            reject(new Error(videoObject.error));
        }
    }, 10000);

    videoElement.src = videoUrl;

  }).catch((error: Error) => {
      if (!videoObject.error) {
          videoObject.error = error.message || "Unknown thumbnail generation error.";
          log(`Caught error for ${videoObject.file.name}: ${videoObject.error}`);
      }
  });
}

// *** MODIFIED: Function to Move Videos ***
async function moveSelectedVideos(): Promise<void> {
  if (selectedVideos.value.length === 0) {
    log("No videos selected to move.");
    return;
  }
  if (!baseDirectory.value) {
      log("Error: Base directory not set.");
      alert("Error: Base directory is not set. Please select it first.");
      return;
  }

  // Construct absolute paths (Unchanged logic)
  const absoluteFilePaths = selectedVideos.value.map(video => {
      const relativePath = video.file.webkitRelativePath || video.file.name;
      // Go backend should use filepath.Join for robustness
      return `${baseDirectory.value}/${relativePath}`.replace(/\\/g, '/');
  });

  log(`Requesting move from Go for ${absoluteFilePaths.length} files:`);
  absoluteFilePaths.forEach(p => log(` - ${p}`));

  // *** MODIFIED: Update status variables ***
  isMoving.value = true;
  moveError.value = '';
  moveSuccessMessage.value = '';

  try {
    // *** MODIFIED: Call MoveVideos Go function ***
    await MoveVideos(absoluteFilePaths);
    log("Move request sent to Go backend. Waiting for response...");
    // Success/error message handling moved to event listeners ("move-complete", "move-error")

  } catch (err: any) {
    const errorText = `Failed to initiate move: ${err}`;
    log(`Error calling Go MoveVideos function: ${errorText}`);
    moveError.value = errorText;
    isMoving.value = false; // Reset moving state on immediate call failure
  }
}

</script>

<style scoped>
/* STYLES MOSTLY UNCHANGED - Added minor styles */
.video-manager { /* ... */ }
button { /* ... */ }
button:disabled { /* ... */ }
button:hover:not(:disabled) { /* ... */ }
.loading { /* ... */ }
.video-list-container { /* ... */ }
.video-list { /* ... */ }
.video-item { /* ... */ }
/* ... rest of the styles ... */

.warning {
    color: #856404;
    background-color: #fff3cd;
    border: 1px solid #ffeeba;
    padding: 5px 10px;
    border-radius: 4px;
    margin: 10px 0;
    display: inline-block;
}

.error-message {
    color: #D8000C;
    background-color: #FFD2D2;
    border: 1px solid #ffbaba;
    padding: 10px;
    border-radius: 4px;
    margin-top: 10px;
}

.success-message {
    color: #4F8A10;
    background-color: #DFF2BF;
     border: 1px solid #bdeca3;
    padding: 10px;
    border-radius: 4px;
    margin-top: 10px;
}

.filename {
  font-size: 0.9em;
  flex-grow: 1;
  word-break: break-all;
  padding-right: 10px;
  color: #333; /* Slightly dimmer text for path */
}

code {
    background-color: #eee;
    padding: 2px 4px;
    border-radius: 3px;
    font-family: monospace;
}
</style>