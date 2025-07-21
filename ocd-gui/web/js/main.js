import { CONFIG } from './constants.js';
import { createWebSocketUrl, generateTimestamp, downloadTextFile, ansiToHtml } from './utils.js';
import { HistoryManager } from './history-manager.js';
import { ProgressManager } from './progress-manager.js';

class OCDApp {
    constructor() {
        this.initializeElements();
        this.initializeManagers();
        this.initializeState();
        this.setupEventListeners();
        this.initialize();
    }

    initializeElements() {
        this.folderInput = document.getElementById('folder-path');
        this.browseBtn = document.getElementById('browse-btn');
        this.deployBtn = document.getElementById('deploy-btn');
        this.statusMessage = document.getElementById('status-message');
        this.manualInputNote = document.getElementById('manual-input-note');
        this.historyDropdown = document.getElementById('history-dropdown');
        this.deploymentSection = document.getElementById('deployment-section');
        this.progressOverview = document.getElementById('progress-overview');
        this.outputWindow = document.getElementById('output-window');
        this.outputContent = document.getElementById('output-content');
        this.toggleOutputBtn = document.getElementById('toggle-output-btn');
        this.saveOutputBtn = document.getElementById('save-output-btn');
        this.progressBarSection = document.getElementById('progress-bar-section');
        this.progressBarFill = document.getElementById('progress-bar-fill');
        this.progressText = document.getElementById('progress-text');
        this.progressPercentage = document.getElementById('progress-percentage');
    }

    initializeManagers() {
        this.historyManager = new HistoryManager(this.historyDropdown, this.folderInput);
        this.progressManager = new ProgressManager(
            this.progressOverview,
            this.progressBarFill,
            this.progressText,
            this.progressPercentage
        );
    }

    initializeState() {
        this.currentDeploymentOutput = '';
        this.websocket = null;
        this.lastDeploymentStatus = null;
    }

    setupEventListeners() {
        // Folder input events
        this.folderInput.addEventListener('click', (e) => this.handleFolderInputClick(e));
        this.folderInput.addEventListener('input', () => this.validateFolderPath());
        this.folderInput.addEventListener('paste', () => {
            setTimeout(() => this.validateFolderPath(), 10);
        });

        // Button events
        this.browseBtn.addEventListener('click', () => this.handleBrowseClick());
        this.deployBtn.addEventListener('click', () => this.handleDeployClick());
        this.toggleOutputBtn.addEventListener('click', () => this.toggleOutput());
        this.saveOutputBtn.addEventListener('click', () => this.saveOutput());

        // Document events
        document.addEventListener('click', (e) => this.handleDocumentClick(e));

        // Window events
        window.addEventListener('load', () => this.handleWindowLoad());

        // Add cleanup on page unload
        window.addEventListener('beforeunload', () => this.cleanup());
        window.addEventListener('unload', () => this.cleanup());
    }

    handleFolderInputClick(e) {
        e.stopPropagation();
        this.historyManager.showHistoryDropdown();
    }

    handleDocumentClick(e) {
        if (!this.folderInput.contains(e.target) && !this.historyDropdown.contains(e.target)) {
            this.historyManager.hideHistoryDropdown();
        }
    }

    handleWindowLoad() {
        if (this.lastDeploymentStatus) {
            const message = this.lastDeploymentStatus === 'success' ?
                'Last deployment completed successfully!' :
                'Last deployment failed';
            this.showStatus(message, this.lastDeploymentStatus, true);
        }
    }

    validateFolderPath() {
        const path = this.folderInput.value.trim();
        this.deployBtn.disabled = !path;
    }

    showStatus(message, type, persistent = false) {
        this.statusMessage.textContent = message;
        this.statusMessage.className = `status-message status-${type}`;
        this.statusMessage.style.display = 'block';

        if (!persistent) {
            setTimeout(() => {
                this.statusMessage.style.display = 'none';
            }, CONFIG.STATUS_DISPLAY_TIME);
        }
    }

    async handleBrowseClick() {
        this.browseBtn.disabled = true;
        this.browseBtn.textContent = 'Browsing...';

        try {
            const response = await fetch('/api/browse');
            const data = await response.json();

            if (data.success) {
                this.folderInput.value = data.folderPath;
                this.validateFolderPath();
                this.historyManager.addToHistory(data.folderPath);
                this.historyManager.hideHistoryDropdown();
                // Removed "Folder selected successfully" message
            } else {
                this.handleBrowseError(data);
            }
        } catch (error) {
            this.handleBrowseError({ message: error.message });
        } finally {
            this.browseBtn.disabled = false;
            this.browseBtn.textContent = 'Browse';
        }
    }

    handleBrowseError(data) {
        if (data.folderPath) {
            this.folderInput.value = data.folderPath;
            this.manualInputNote.style.display = 'block';
            this.validateFolderPath();
            this.showStatus(data.message + ' Please verify or edit the path above.', 'warning');
        } else if (data.message) {
            this.manualInputNote.style.display = 'block';
            this.showStatus(data.message, 'error');
        }
    }

    handleDeployClick() {
        if (!this.folderInput.value.trim()) {
            this.showStatus('Please select a project folder first', 'error');
            return;
        }

        this.lastDeploymentStatus = null;
        this.statusMessage.style.display = 'none';

        // Show deployment sections
        this.deploymentSection.style.display = 'block';
        this.progressBarSection.style.display = 'block';
        this.progressOverview.style.display = 'block';
        
        // Clear previous output and reset progress
        this.clearOutput();
        this.progressManager.reset();
        this.progressManager.initialize(); // Add this line

        this.deployBtn.disabled = true;
        this.deployBtn.textContent = 'Deploying...';

        const folderPath = this.folderInput.value.trim();

        // Debug the WebSocket URL
        const wsUrl = createWebSocketUrl();
        console.log('WebSocket URL:', wsUrl);

        // Create WebSocket connection
        this.websocket = new WebSocket(wsUrl);

        this.websocket.onopen = () => {
            console.log('WebSocket connected');
            // Send the folder path
            this.websocket.send(JSON.stringify({ folderPath: folderPath }));
        };

        this.websocket.onmessage = (event) => this.handleWebSocketMessage(event);
        this.websocket.onclose = () => this.handleWebSocketClose();
        this.websocket.onerror = (error) => this.handleWebSocketError(error);
    }

    handleWebSocketMessage(event) {
        try {
            const data = JSON.parse(event.data);
            console.log('Received WebSocket message:', data); // Debug log

            switch (data.type) {
                case 'output':
                    console.log('Processing output:', data.content); // Debug log
                    this.appendOutput(data.content);
                    break;

                case 'progress':
                    console.log('Processing progress:', data); // Debug log
                    this.progressManager.handleProgressUpdate(data);
                    break;

                case 'complete':
                    console.log('Processing completion:', data); // Debug log
                    this.handleDeploymentComplete(data);
                    break;

                default:
                    console.warn('Unknown message type:', data.type);
            }
        } catch (error) {
            console.error('Error parsing WebSocket message:', error);
            this.showStatus('Error processing server response', 'error');
            this.resetDeployButton();
        }
    }

    handleWebSocketError(error) {
        console.error('WebSocket error:', error);
        this.showStatus('Connection error occurred', 'error');
        this.resetDeployButton();
        this.cleanup();
    }

    handleWebSocketClose() {
        console.log('WebSocket connection closed');
        // Only show error if deployment was still in progress
        if (this.deployBtn.textContent === 'Deploying...') {
            this.resetDeployButton();
            this.showStatus('Connection lost during deployment', 'error');
            this.cleanup(); // Only cleanup on error
        }
        // Just close the websocket connection, don't reset progress
        this.websocket = null;
    }

    handleDeploymentComplete(data) {
        this.lastDeploymentStatus = data.success ? 'success' : 'error';
        const message = data.success ?
            'Deployment completed successfully!' :
            `Deployment failed: ${data.content}`;

        if (data.success) {
            this.progressManager.updateProgressBar(100, 'Deployment completed successfully!');
            this.historyManager.addToHistory(this.folderInput.value.trim());
            this.progressManager.updateStageProgress('patch', 'success');
        } else {
            this.progressManager.updateProgressBar(0, 'Deployment failed');
        }

        this.showStatus(message, this.lastDeploymentStatus, true);
        this.resetDeployButton();
    }

    resetDeployButton() {
        this.deployBtn.disabled = false;
        this.deployBtn.textContent = 'Deploy Changes';
        this.websocket = null;
    }

    appendOutput(content, type = 'normal') {
        const coloredContent = ansiToHtml(content);
        const line = document.createElement('div');
        line.className = `output-line ${type}`;
        line.innerHTML = coloredContent; // Use innerHTML instead of textContent
        this.outputContent.appendChild(line);
        this.outputContent.scrollTop = this.outputContent.scrollHeight;

        // For saving, still use clean content
        const cleanContent = content.replace(/\x1b\[[0-9;]*m/g, '');
        this.currentDeploymentOutput += cleanContent + '\n';
        this.saveOutputBtn.disabled = false;
    }

    clearOutput() {
        this.outputContent.innerHTML = '';
        this.currentDeploymentOutput = '';
        this.saveOutputBtn.disabled = true;
    }

    toggleOutput() {
        const isVisible = this.outputWindow.style.display !== 'none';

        if (isVisible) {
            this.outputWindow.style.display = 'none';
            this.toggleOutputBtn.textContent = 'Show Output';
        } else {
            this.outputWindow.style.display = 'block';
            this.toggleOutputBtn.textContent = 'Hide Output';
        }
    }

    saveOutput() {
        if (!this.currentDeploymentOutput) return;

        const timestamp = generateTimestamp();
        const filename = `ocd-deployment-${timestamp}.txt`;
        downloadTextFile(this.currentDeploymentOutput, filename);
    }

    initialize() {
        this.validateFolderPath();
        this.addDeveloperCredit();
        this.initializeTheme();
    }

    initializeTheme() {
        const savedTheme = localStorage.getItem('theme') || 'light';
        document.documentElement.setAttribute('data-theme', savedTheme);

        const themeToggle = document.querySelector('.theme-toggle');
        if (themeToggle) {
            themeToggle.textContent = savedTheme === 'dark' ? '☀️ Light' : '🌙 Dark';
        }
    }

    addDeveloperCredit() {
        const footer = document.createElement('div');
        footer.className = 'developer-credit';
        footer.innerHTML = `
        <span>Developed by Shadee Nicola</span>
        <span>•</span>
        <span>V1.0</span>
        <button class="mail-btn" onclick="sendMail()" title="Contact Developer">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor">
                <path d="M20 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4l-8 5-8-5V6l8 5 8-5v2z"/>
            </svg>
        </button>
    `;
        document.body.appendChild(footer);
    }

    cleanup() {
        if (this.websocket) {
            this.websocket.close();
            this.websocket = null;
        }
    }
}

// Global functions for HTML onclick handlers
window.toggleTheme = function() {
    const currentTheme = document.documentElement.getAttribute('data-theme');
    const newTheme = currentTheme === 'dark' ? 'light' : 'dark';

    document.documentElement.setAttribute('data-theme', newTheme);
    localStorage.setItem('theme', newTheme);

    const themeToggle = document.querySelector('.theme-toggle');
    themeToggle.textContent = newTheme === 'dark' ? '☀️ Light' : '🌙 Dark';
};

window.sendMail = function() {
    const subject = encodeURIComponent('OCD Tool - Contact');
    const body = encodeURIComponent('Hello Shadee,\n\nI am contacting you regarding the OCD (One Click Deployer) tool.\n\n');
    window.open(`mailto:shadee.nicola@amdocs.com?subject=${subject}&body=${body}`);
};

// Initialize the application
new OCDApp();