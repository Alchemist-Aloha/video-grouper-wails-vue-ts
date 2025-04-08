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

    <input type="file" webkitdirectory multiple @change="handleFolderSelect" ref="folderInput" style="display: none;"
      accept="video/*" />
    <button @click="triggerFolderSelect" :disabled="!baseDirectory">
      Load Videos from Selected Directory
    </button>

    <div v-if="isLoading" class="loading">Generating thumbnails...</div>

    <div v-if="videos.length > 0" class="video-list-container">
      <h3>Loaded Videos:</h3>
      <ul class="video-list">
        <li v-for="video in videos" :key="video.id" class="video-item">
          <input type="checkbox" :id="'video-' + video.id" v-model="video.selected" />
          <label :for="'video-' + video.id">
            <img v-if="video.thumbnail" :src="video.thumbnail" alt="Video thumbnail" class="thumbnail" />
            <div v-else class="thumbnail placeholder">No thumbnail</div>
            <span class="filename">
              {{ video.file.webkitRelativePath || video.file.name }}
            </span>
            <span v-if="video.error" class="error-tag">Error</span>
          </label>
        </li>
      </ul>

      <button @click="moveSelectedVideos" :disabled="selectedVideos.length === 0 || !baseDirectory || isMoving">
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
        <li>The new folder (named after the first selected video) will be created inside the *parent* of your selected
          Base Directory.</li>
        <li>**Warning:** This action moves the original files. Ensure you have backups if needed.</li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue';
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime';
import { SelectDirectory, MoveVideos, GenerateThumbnail } from '../wailsjs/go/main/App';

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
const isMoving = ref<boolean>(false);
const moveError = ref<string>('');
const moveSuccessMessage = ref<string>('');
const movingVideoIds = ref<number[]>([]); // Keep track of IDs being moved

let videoIdCounter: number = 0;

// --- Computed Properties ---
const selectedVideos = computed<VideoItem[]>(() => {
  return videos.filter(video => video.selected);
});

// --- Wails Event Handling ---
onMounted(() => {
  EventsOn("move-status", (status: string) => {
    log(`[Go Backend] ${status}`);
  });
  EventsOn("move-complete", (successMsg: string) => {
    log(`[Go Backend] ${successMsg}`);
    moveSuccessMessage.value = successMsg;
    isMoving.value = false;
    moveError.value = '';
    // Remove moved videos from the list
    movingVideoIds.value.forEach(movedId => {
      const index = videos.findIndex(v => v.id === movedId);
      if (index !== -1) {
        videos.splice(index, 1);
      }
    });
    movingVideoIds.value = []; // Clear the tracking array
  });
  EventsOn("move-error", (errorMsg: string) => {
    log(`[Go Backend] Move Error: ${errorMsg}`);
    moveError.value = errorMsg;
    isMoving.value = false;
    moveSuccessMessage.value = '';
    movingVideoIds.value = []; // Clear the tracking array on error too
  });
});

onBeforeUnmount(() => {
  EventsOff("move-status");
  EventsOff("move-complete");
  EventsOff("move-error");
});

// --- Methods ---

function log(message: string): void {
  console.log(message);
  const timestamp = new Date().toLocaleTimeString();
  logMessages.value.push(`[${timestamp}] ${message}`);
  if (logMessages.value.length > 100) {
    logMessages.value.shift();
  }
}

async function selectBaseDirectory() {
  log("Requesting base directory selection from Go...");
  try {
    const videoPaths: string[] = await SelectDirectory();
    if (videoPaths && videoPaths.length > 0) {
      baseDirectory.value = videoPaths[0].substring(0, videoPaths[0].lastIndexOf('\\'));
      log(`Base directory set: ${baseDirectory.value}`);
      videos.splice(0, videos.length);
      logMessages.value = [];

      log(`Found ${videoPaths.length} video files. Generating thumbnails...`);
      videoPaths.forEach((path) => {
        videos.push({
          id: videoIdCounter++,
          file: { name: path.split('/').pop() || '', webkitRelativePath: path } as File,
          thumbnail: null,
          selected: false,
          error: null,
        });
      });

      await chunkGenerateThumbnails(videos);
      log("Thumbnail generation complete.");
    } else {
      log("No videos found or directory selection cancelled.");
    }
  } catch (err: any) {
    log(`Error selecting directory: ${err}`);
    alert(`Error selecting directory: ${err}`);
  }
}

function triggerFolderSelect() {
  if (!baseDirectory.value) {
    alert("Please select the base directory first!");
    return;
  }
  logMessages.value = [];
  folderInput.value?.click();
}

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
  videos.splice(0, videos.length);

  const videoFiles: File[] = Array.from(files).filter(file => file.type.startsWith('video/'));

  if (videoFiles.length === 0) {
    log("No video files found matching the input files.");
    isLoading.value = false;
    if (target) target.value = '';
    return;
  }

  log(`Found ${videoFiles.length} video files from input. Generating thumbnails...`);

  videoFiles.forEach(file => {
    const relativePath = file.webkitRelativePath || file.name;
    log(`Processing file: ${relativePath}`);

    videos.push({
      id: videoIdCounter++,
      file: file,
      thumbnail: null,
      selected: false,
      error: null,
    });
  });

  await chunkGenerateThumbnails(videos);

  isLoading.value = false;
  log("Thumbnail generation complete.");
  if (target) target.value = '';
}

async function generateThumbnail(videoObject: VideoItem): Promise<void> {
  try {
    // if (!baseDirectory.value) {
    //   throw new Error("Base directory is not set.");
    // }

    // const videoPath = videoObject.file.webkitRelativePath || videoObject.file.name;
    const fullVideoPath = videoObject.file.name;

    log(`Requesting Go backend to generate thumbnail for: ${fullVideoPath}`);
    const thumbnailDataUrl = await GenerateThumbnail(fullVideoPath);

    // Ensure the thumbnail path is set correctly
    videoObject.thumbnail = thumbnailDataUrl;
    log(`Thumbnail generated for ${videoObject.file.name}`);
  } catch (error: any) {
    videoObject.error = `Failed to generate thumbnail: ${error.message}`;
    log(`Error generating thumbnail for ${videoObject.file.name}: ${videoObject.error}`);
  }
}

async function chunkGenerateThumbnails(videoArray: VideoItem[], chunkSize = 2) {
  for (let i = 0; i < videoArray.length; i += chunkSize) {
    const chunk = videoArray.slice(i, i + chunkSize);
    await Promise.all(chunk.map(generateThumbnail));
    await new Promise(res => setTimeout(res, 0)); // Let the UI breathe
  }
}

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

  // Store the IDs of the videos we are about to move
  movingVideoIds.value = selectedVideos.value.map(video => video.id);

  const absoluteFilePaths = selectedVideos.value.map(video => {
    // Use the full path stored when loading videos
    const fullPath = video.file.name; // Assuming file.name holds the full path from SelectDirectory
    return fullPath;
  });


  log(`Requesting move from Go for ${absoluteFilePaths.length} files:`);
  absoluteFilePaths.forEach(p => log(` - ${p}`));

  isMoving.value = true;
  moveError.value = '';
  moveSuccessMessage.value = '';

  try {
    await MoveVideos(absoluteFilePaths);
    log("Move request sent to Go backend. Waiting for response...");
  } catch (err: any) {
    const errorText = `Failed to initiate move: ${err}`;
    log(`Error calling Go MoveVideos function: ${errorText}`);
    moveError.value = errorText;
    isMoving.value = false;
    movingVideoIds.value = []; // Clear IDs if the call itself fails
  }
}

</script>

<style scoped>
.video-manager {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 20px;
}

button {
  padding: 10px 20px;
  font-size: 16px;
  cursor: pointer;
}

button:disabled {
  background-color: #ccc;
  cursor: not-allowed;
}

button:hover:not(:disabled) {
  background-color: #007bff;
  color: white;
}

.loading {
  font-size: 18px;
  color: #007bff;
}

.video-list-container {
  margin-top: 20px;
  width: 100%;
}

.video-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 16px;
  list-style: none;
  padding: 0;
  margin: 10px 0;
}

.video-item {
  border: 1px solid #ccc;
  border-radius: 4px;
  overflow: hidden;
  background-color: #fafafa;
  padding: 10px;
  text-align: center;
}

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
  color: #333;
}

code {
  background-color: #eee;
  padding: 2px 4px;
  border-radius: 3px;
  font-family: monospace;
}

.thumbnail {
  width: 100px;
  /* Set the desired width */
  height: 100px;
  /* Set the desired height */
  object-fit: cover;
  /* Ensures the image fits within the dimensions without distortion */
  border-radius: 4px;
  /* Optional: Add rounded corners */
  margin-bottom: 8px;
  /* Add spacing below the thumbnail */
}

.thumbnail.placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #ccc;
  color: #666;
  font-size: 12px;
  width: 100px;
  height: 100px;
  border-radius: 4px;
}
</style>