import { exec } from "yt-dlp-exec";
import axios from "axios";
import FormData from "form-data";
import fs from "fs";
import path from "path";

async function downloadAudio(youtubeUrl: string, outputPath: string): Promise<void> {
    const options = {
        output: outputPath,
        extractAudio: true,
        audioFormat: "mp3",
        audioQuality: 0, // Best quality
    };

    try {
        await exec(youtubeUrl, options);
        console.log("Audio downloaded successfully.");
    } catch (error) {
        console.error("Error downloading audio:", error);
    }
}

async function sendAudioToLemonfox(filePath: string, apiKey: string): Promise<void> {
    const url = "https://api.lemonfox.ai/v1/audio/transcriptions";
    const formData = new FormData();
    
    // Read the file into a Blob first, as per Lemonfox's example
    const fileBuffer = await fs.promises.readFile(filePath);
    formData.append('file', new Blob([fileBuffer]));
    formData.append('language', 'english');
    formData.append('response_format', 'json');

    try {
        const response = await axios.post(url, formData, {
            headers: {
                'Authorization': `Bearer ${apiKey}`
                // Remove formData.getHeaders() as it's not needed with Blob
            },
        });
        
        // Access the text property directly from response.data
        console.log("Transcription result:", response.data.text);
    } catch (error) {
        console.error("Error sending audio to Lemonfox:", error);
        throw error; // Re-throw the error to handle it in the calling function
    }
}

async function main() {
    const youtubeUrl = "https://www.youtube.com/watch?v=Y9QfOPxmxVI"; // Replace with the actual YouTube URL
    const outputPath = path.resolve(
        path.dirname(new URL(import.meta.url).pathname),
        "downloaded_audio.mp3"
    );
    const apiKey = "YOUR_API_KEY"; // Replace with your actual Lemonfox API key

    await downloadAudio(youtubeUrl, outputPath);
    await sendAudioToLemonfox(outputPath, apiKey);
}

main();