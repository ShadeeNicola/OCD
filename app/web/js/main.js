import { CONFIG } from './constants.js';
import { generateTimestamp, downloadTextFile, ansiToHtml } from './utils.js';
import { HistoryManager } from './history-manager.js';
import { ProgressManager } from './progress-manager.js';
import { initializeScaling } from './jenkins-scaling.js';
import { initializeSettings } from './settings.js';
import { initializeClusterSelector } from './cluster-selector.js';

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
        this.eventSource = null;
        this.currentSessionId = null;
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
        this.deployBtn.addEventListener('click', () => this.handleDeployButton());
        this.toggleOutputBtn.addEventListener('click', () => this.toggleOutput());
        this.saveOutputBtn.addEventListener('click', () => this.saveOutput());

        // Navigation events
        this.setupNavigation();

        // Document events
        document.addEventListener('click', (e) => this.handleDocumentClick(e));

        // Window events
        window.addEventListener('load', () => this.handleWindowLoad());

        // Add cleanup on page unload
        window.addEventListener('beforeunload', () => this.cleanup());
        window.addEventListener('unload', () => this.cleanup());
    }

    setupNavigation() {
        const navLinks = document.querySelectorAll('.nav-link');
        navLinks.forEach(link => {
            link.addEventListener('click', (e) => {
                e.preventDefault();
                const targetPage = link.getAttribute('data-page');
                this.navigateToPage(targetPage);
            });
        });
    }

    navigateToPage(pageName) {
        // Remove active class from all nav links and pages
        document.querySelectorAll('.nav-link').forEach(link => {
            link.classList.remove('active');
        });
        document.querySelectorAll('.page').forEach(page => {
            page.classList.remove('active');
        });

        // Add active class to clicked nav link
        const activeLink = document.querySelector(`[data-page="${pageName}"]`);
        if (activeLink) {
            activeLink.classList.add('active');
        }

        // Show target page
        const targetPage = document.getElementById(`${pageName}-page`);
        if (targetPage) {
            targetPage.classList.add('active');
        }


        // Initialize page-specific functionality
        this.initializePageFunctionality(pageName);
    }

    initializePageFunctionality(pageName) {
        // Initialize functionality based on the active page
        switch (pageName) {
            case 'scaling':
                // Re-initialize cluster selector when scaling page is active
                initializeClusterSelector();
                break;
            case 'settings':
                // Re-initialize settings when settings page is active
                initializeSettings();
                break;
        }
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

    async handleDeployButton() {
        // If deploying, clicking acts as cancel
        if (this.deployBtn.dataset.state === 'deploying') {
            this.handleCancelClick();
            return;
        }

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
        this.progressManager.initialize();

        this.setDeployingState(true);

        const folderPath = this.folderInput.value.trim();

        try {
            // Start deployment session
            const response = await fetch('/api/deploy/start', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ folderPath: folderPath })
            });

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            const data = await response.json();
            this.currentSessionId = data.sessionId;
            console.log('Deployment session started:', this.currentSessionId);

            // Connect to SSE stream
            this.eventSource = new EventSource(`/api/deploy/stream/${this.currentSessionId}`);
            this.eventSource.onmessage = (event) => this.handleSSEMessage(event);
            this.eventSource.onerror = (error) => this.handleSSEError(error);
            
        } catch (error) {
            console.error('Failed to start deployment:', error);
            this.showStatus(`Failed to start deployment: ${error.message}`, 'error');
            this.resetDeployButton();
        }
    }

    async handleCancelClick() {
        if (!this.currentSessionId) return;
        
        try {
            const response = await fetch(`/api/deploy/cancel/${this.currentSessionId}`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' }
            });
            
            if (response.ok) {
                this.showStatus('Aborting deployment...', 'warning');
            } else {
                console.error('Failed to cancel deployment:', response.statusText);
            }
        } catch (error) {
            console.error('Error cancelling deployment:', error);
        }
    }

    handleSSEMessage(event) {
        try {
            const data = JSON.parse(event.data);
            console.log('Received SSE message:', data);

            switch (data.type) {
                case 'connected':
                    console.log('SSE connected for session:', data.sessionId);
                    break;
                    
                case 'output':
                    console.log('Processing output:', data.content);
                    this.appendOutput(data.content);
                    break;

                case 'progress':
                    console.log('Processing progress:', data);
                    this.progressManager.handleProgressUpdate(data);
                    break;

                case 'complete':
                    console.log('Processing completion:', data);
                    this.handleDeploymentComplete(data);
                    break;
                    
                case 'keepalive':
                    // Ignore keepalive messages
                    break;

                default:
                    console.warn('Unknown message type:', data.type);
            }
        } catch (error) {
            console.error('Error parsing SSE message:', error);
            this.showStatus('Error processing server response', 'error');
            this.resetDeployButton();
        }
    }

    handleSSEError(error) {
        console.error('SSE error:', error);
        this.showStatus('Connection error occurred', 'error');
        this.resetDeployButton();
        this.cleanup();
    }

    handleSSEClose() {
        console.log('SSE connection closed');
        // Only show error if deployment was still in progress
        if (this.deployBtn.textContent === 'Deploying...Click to Abort!') {
            this.resetDeployButton();
            this.showStatus('Connection lost during deployment', 'error');
            this.cleanup();
        }
        // Clean up the event source
        this.eventSource = null;
        this.currentSessionId = null;
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
        this.setDeployingState(false);
        this.cleanup();
    }

    setDeployingState(isDeploying) {
        if (isDeploying) {
            this.deployBtn.dataset.state = 'deploying';
            this.deployBtn.classList.add('danger');
            this.deployBtn.textContent = 'Deploying...Click to Abort!';
        } else {
            this.deployBtn.dataset.state = '';
            this.deployBtn.classList.remove('danger');
            this.deployBtn.textContent = 'Deploy Changes';
            this.deployBtn.disabled = false;
        }
    }

    appendOutput(content, type = 'normal') {
        const coloredContent = ansiToHtml(content);
        const line = document.createElement('div');
        line.className = `output-line ${type}`;
        line.innerHTML = coloredContent;
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
        // Initialize Jenkins scaling functionality (for the scaling page)
        initializeScaling();
        // fetch version info and update UI footer
        fetch('/api/health').then(r => r.json()).then(info => {
            const el = document.getElementById('app-version');
            if (el && info && info.version) {
                el.textContent = info.version;
                if (info.commit) el.title = `commit: ${info.commit}  built: ${info.date || ''}`;
            }
        }).catch(() => {});
        
        // Ensure landing page is active by default
        this.navigateToPage('landing');
    }

    cleanup() {
        if (this.eventSource) {
            this.eventSource.close();
            this.eventSource = null;
        }
        this.currentSessionId = null;
    }
}

// Global functions for HTML onclick handlers
window.sendMail = function() {
    const subject = encodeURIComponent('OCD Tool - Contact');
    window.open(`mailto:shadee.nicola@amdocs.com?subject=${subject}&body=${body}`);
};

// Initialize the application
new OCDApp();