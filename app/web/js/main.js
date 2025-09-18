import { CONFIG } from './constants.js';
import { generateTimestamp, downloadTextFile, ansiToHtml } from './utils.js';
import { HistoryManager } from './history-manager.js';
import { ProgressManager } from './progress-manager.js';
import { initializeScaling } from './jenkins-scaling.js';
import { initializeRNCreation, enableTriggerButton } from './jenkins-rn-creation.js';
import { initializeSettings, getSavedBitbucketCredentials, getSavedCredentials } from './settings.js';
import { initializeClusterSelector } from './cluster-selector.js';
import { initializeHFAdoption } from './hf-adoption.js';

class OCDApp {
    constructor() {
        this.initializeElements();
        this.initializeManagers();
        this.initializeState();
        this.setupEventListeners();
        this.initialize();
        
        // Make instance available globally for modules to access
        window.ocdApp = this;
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
            case 'rn-creation':
                // Re-initialize RN creation when page is active
                this.initializeRNCreation();
                initializeRNCreation();
                break;
            case 'hf-adoption':
                // Initialize HF Adoption page
                initializeHFAdoption();
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

    initializeRNCreation() {
        // Initialize RN Creation page elements
        const branchSelector = document.getElementById('branch-selector');
        const branchSearch = document.getElementById('branch-search');
        const branchDropdownBtn = document.getElementById('branch-dropdown-btn');
        const branchDropdown = document.getElementById('branch-dropdown');
        const refreshBranchesBtn = document.getElementById('refresh-branches-btn');
        const createRNBtn = document.getElementById('create-rn-btn');
        const refreshJobBtn = document.getElementById('refresh-job-btn');
        const customizationJobUrl = document.getElementById('customization-job-url');

        if (branchSelector && branchSearch && branchDropdownBtn && branchDropdown && refreshBranchesBtn && createRNBtn) {
            // Initialize state
            this.selectedBranch = null;
            this.allBranches = [];
            this.isDropdownOpen = false;

            // Load branches on page load
            this.loadBranches();

            // Set up event listeners
            refreshBranchesBtn.addEventListener('click', () => this.loadBranches());
            branchSearch.addEventListener('click', () => this.toggleBranchDropdown());
            branchDropdownBtn.addEventListener('click', () => this.toggleBranchDropdown());
            // Click handler now managed by jenkins-rn-creation.js module

            // Add refresh job button listener
            if (refreshJobBtn) {
                refreshJobBtn.addEventListener('click', () => {
                    if (this.selectedBranch) {
                        this.loadCustomizationJob(this.selectedBranch.name);
                    }
                });
            }

            // Add customization job URL change listener
            if (customizationJobUrl) {
                customizationJobUrl.addEventListener('change', (e) => {
                    if (e.target.value) {
                        this.populateFieldsFromJob(e.target.value);
                    }
                });
            }

            // Close dropdown when clicking outside
            document.addEventListener('click', (e) => {
                if (!branchSelector.contains(e.target)) {
                    this.closeBranchDropdown();
                }
            });
        }
    }

    async loadBranches() {
        const branchSearch = document.getElementById('branch-search');
        const branchLoading = document.getElementById('branch-loading');
        const branchError = document.getElementById('branch-error');
        const branchList = document.getElementById('branch-list');
        const refreshBtn = document.getElementById('refresh-branches-btn');
        
        if (!branchSearch || !refreshBtn) return;

        // Set loading state
        refreshBtn.disabled = true;
        branchSearch.placeholder = 'Loading branches...';
        branchSearch.disabled = true;
        
        if (branchLoading) branchLoading.style.display = 'block';
        if (branchError) branchError.style.display = 'none';
        if (branchList) branchList.innerHTML = '';

        try {
            // Get saved Bitbucket credentials
            const credentials = getSavedBitbucketCredentials();
            
            const headers = {};
            if (credentials && credentials.username && credentials.token) {
                headers['X-Bitbucket-Username'] = credentials.username;
                headers['X-Bitbucket-Token'] = credentials.token;
            }

            const response = await fetch('/api/git/branches/customization', {
                method: 'GET',
                headers: headers
            });
            const data = await response.json();

            if (data.success && data.branches) {
                this.allBranches = data.branches;
                this.populateBranchDropdown();
                branchSearch.placeholder = 'Select a branch...';
            } else {
                if (data.requiresAuth) {
                    branchSearch.placeholder = 'Configure Bitbucket credentials in Settings';
                    this.showBranchError('Configure Bitbucket credentials in Settings');
                } else {
                    branchSearch.placeholder = 'Failed to load branches';
                    this.showBranchError(data.message || 'Failed to load branches');
                }
                console.error('Failed to load branches:', data.message);
            }
        } catch (error) {
            branchSearch.placeholder = 'Error loading branches';
            this.showBranchError(error.message || 'Error loading branches');
            console.error('Error loading branches:', error);
        } finally {
            refreshBtn.disabled = false;
            branchSearch.disabled = false;
            if (branchLoading) branchLoading.style.display = 'none';
        }
    }

    populateBranchDropdown() {
        const branchList = document.getElementById('branch-list');
        if (!branchList) return;

        branchList.innerHTML = '';

        this.allBranches.forEach(branch => {
            const option = document.createElement('div');
            option.className = 'branch-option';
            option.setAttribute('data-branch', branch.name);
            
            option.innerHTML = `
                <div class="branch-name">${branch.name}</div>
            `;

            option.addEventListener('click', () => this.selectBranch(branch));
            branchList.appendChild(option);
        });
    }

    selectBranch(branch) {
        this.selectedBranch = branch;
        
        const branchSearch = document.getElementById('branch-search');
        const selectedBranch = document.getElementById('selected-branch');
        const createRNBtn = document.getElementById('create-rn-btn');
        const customizationJobSection = document.getElementById('customization-job-section');
        const branchNameInput = document.getElementById('branch-name');

        if (branchSearch) {
            branchSearch.value = '';
            branchSearch.placeholder = `Selected: ${branch.name}`;
        }

        if (selectedBranch) {
            selectedBranch.textContent = branch.name;
            selectedBranch.style.display = 'block';
        }

        if (branchNameInput) {
            // Map branch name: remove "release/" prefix or any "*/" prefix
            let mappedBranchName = branch.name;
            if (mappedBranchName.includes('/')) {
                mappedBranchName = mappedBranchName.split('/').pop();
            }
            branchNameInput.value = mappedBranchName;
        }

        if (customizationJobSection) {
            customizationJobSection.style.display = 'block';
            this.loadCustomizationJob(branch.name);
        }

        if (createRNBtn) {
            createRNBtn.disabled = false;
        }
        
        // Enable trigger button in RN Creation module
        enableTriggerButton();

        this.closeBranchDropdown();
    }

    showBranchError(message) {
        const branchError = document.getElementById('branch-error');
        const branchRetryBtn = document.getElementById('branch-retry-btn');
        
        if (branchError) {
            branchError.style.display = 'block';
            const errorText = branchError.querySelector('span');
            if (errorText) errorText.textContent = message;
        }

        if (branchRetryBtn) {
            branchRetryBtn.addEventListener('click', () => this.loadBranches());
        }
    }

    toggleBranchDropdown() {
        if (this.isDropdownOpen) {
            this.closeBranchDropdown();
        } else {
            this.openBranchDropdown();
        }
    }

    openBranchDropdown() {
        const branchDropdown = document.getElementById('branch-dropdown');
        const branchDropdownBtn = document.getElementById('branch-dropdown-btn');
        
        if (branchDropdown && this.allBranches.length > 0) {
            branchDropdown.style.display = 'block';
            this.isDropdownOpen = true;
            
            if (branchDropdownBtn) {
                branchDropdownBtn.classList.add('active');
            }
        }
    }

    closeBranchDropdown() {
        const branchDropdown = document.getElementById('branch-dropdown');
        const branchDropdownBtn = document.getElementById('branch-dropdown-btn');
        
        if (branchDropdown) {
            branchDropdown.style.display = 'none';
            this.isDropdownOpen = false;
            
            if (branchDropdownBtn) {
                branchDropdownBtn.classList.remove('active');
            }
        }
    }

    async loadCustomizationJob(branchName) {
        const customizationJobUrl = document.getElementById('customization-job-url');
        const refreshJobBtn = document.getElementById('refresh-job-btn');
        
        if (!customizationJobUrl || !refreshJobBtn) return;

        // Set loading state
        refreshJobBtn.disabled = true;
        customizationJobUrl.placeholder = 'Loading latest job...';
        customizationJobUrl.value = '';

        try {
            // Get saved Jenkins credentials for Jenkins API calls
            const jenkinsCredentials = getSavedCredentials();
            
            // Follow the same pattern as scaling - use query params for GET requests
            let url = `/api/jenkins/rn-customization-job?branch=${encodeURIComponent(branchName)}`;
            if (jenkinsCredentials && jenkinsCredentials.username && jenkinsCredentials.token) {
                url += `&username=${encodeURIComponent(jenkinsCredentials.username)}&token=${encodeURIComponent(jenkinsCredentials.token)}`;
            }

            const response = await fetch(url, {
                method: 'GET'
            });
            const data = await response.json();

            if (data.success && data.job) {
                customizationJobUrl.value = data.job.url;
                customizationJobUrl.placeholder = 'Latest customization job URL';
                
                // Auto-populate other fields from the job
                await this.populateFieldsFromJob(data.job.url);
            } else {
                customizationJobUrl.placeholder = 'Failed to load job';
                customizationJobUrl.title = data.message || 'Failed to load job';
                console.error('Failed to load customization job:', data.message);
                
                // Show error message to user if Jenkins credentials are missing
                if (data.message && data.message.includes('credentials')) {
                    const statusElement = document.getElementById('rn-creation-status');
                    const statusText = document.getElementById('rn-creation-status-text');
                    if (statusElement && statusText) {
                        statusElement.style.display = 'block';
                        statusText.textContent = 'Jenkins credentials required. Please configure them in Settings.';
                    }
                }
            }
        } catch (error) {
            customizationJobUrl.placeholder = 'Error loading job';
            customizationJobUrl.title = error.message || 'Error loading job';
            console.error('Error loading customization job:', error);
            
            // Show error message to user
            const statusElement = document.getElementById('rn-creation-status');
            const statusText = document.getElementById('rn-creation-status-text');
            if (statusElement && statusText) {
                statusElement.style.display = 'block';
                statusText.textContent = 'Error loading customization job: ' + (error.message || 'Unknown error');
            }
        } finally {
            refreshJobBtn.disabled = false;
        }
    }

    async populateFieldsFromJob(jobUrl) {
        
        try {
            // Get saved credentials
            const jenkinsCredentials = getSavedCredentials();
            const bitbucketCredentials = getSavedBitbucketCredentials();
            
            
            const buildChartVersionInput = document.getElementById('build-chart-version');
            const customOrchZipUrlInput = document.getElementById('custom-orch-zip-url');
            const oniImageInput = document.getElementById('oni-image');
            const emailInput = document.getElementById('email');
            const coreVersionSelect = document.getElementById('core-version');
            
            // Auto-populate email from Bitbucket credentials
            if (emailInput && bitbucketCredentials && bitbucketCredentials.username) {
                emailInput.value = bitbucketCredentials.username + '@amdocs.com';
            }
            
            if (!jenkinsCredentials || !jenkinsCredentials.username || !jenkinsCredentials.token) {
                return;
            }
            
            // 1. Get build parameters to extract release_version and TLC version
            await this.fetchBuildParameters(jobUrl, jenkinsCredentials, coreVersionSelect, buildChartVersionInput);
            
            // 2. Get artifact URL
            await this.fetchArtifactURL(jobUrl, jenkinsCredentials, customOrchZipUrlInput);
            
            // 3. Get ONI image from Bitbucket
            if (bitbucketCredentials && bitbucketCredentials.username && bitbucketCredentials.token) {
                await this.fetchOniImage(bitbucketCredentials, oniImageInput);
            }
            
        } catch (error) {
            console.error('Error populating fields from job:', error);
        }
    }
    
    async fetchBuildParameters(jobUrl, credentials, coreVersionSelect, buildChartVersionInput) {
        try {
            
            // Call backend API to get build parameters
            const apiUrl = `/api/jenkins/rn-build-parameters?job_url=${encodeURIComponent(jobUrl)}&username=${encodeURIComponent(credentials.username)}&token=${encodeURIComponent(credentials.token)}`;
            
            const response = await fetch(apiUrl);
            const data = await response.json();
            
            
            if (data.success && data.parameters) {
                // Look for release_version parameter
                const releaseVersionParam = data.parameters.find(p => p.name === 'release_version');
                if (releaseVersionParam && releaseVersionParam.value) {
                    // Map the release version format to storage job format
                    const mappedVersion = this.mapReleaseVersionToStorageFormat(releaseVersionParam.value);
                    if (mappedVersion) {
                        coreVersionSelect.value = mappedVersion;
                    }
                }
                
                // Get TLC version from build description instead of parameters
                if (data.description && buildChartVersionInput) {
                    
                    // Extract TLC version from description like "TLC Version = 10.4.86-PI24-DROP2"
                    const tlcMatch = data.description.match(/TLC Version\s*=\s*([^<\s]+)/);
                    if (tlcMatch && tlcMatch[1]) {
                        buildChartVersionInput.value = tlcMatch[1];
                    } else {
                    }
                }
            } else {
            }
        } catch (error) {
            console.error('Error fetching build parameters:', error);
        }
    }
    
    async fetchArtifactURL(jobUrl, credentials, customOrchZipUrlInput) {
        try {
            // Include branch parameter for Nexus fallback
            const branch = this.selectedBranch ? this.selectedBranch.name : '';
            console.log(`Fetching artifact URL for branch: ${branch}`);
            const apiUrl = `/api/jenkins/rn-artifact-url?job_url=${encodeURIComponent(jobUrl)}&username=${encodeURIComponent(credentials.username)}&token=${encodeURIComponent(credentials.token)}&branch=${encodeURIComponent(branch)}`;

            const response = await fetch(apiUrl);
            const data = await response.json();

            console.log('Artifact URL response:', data);

            if (data.success && data.artifact_url && customOrchZipUrlInput) {
                customOrchZipUrlInput.value = data.artifact_url;
                console.log(`Custom Orchestration ZIP URL set to: ${data.artifact_url}`);
            } else if (!data.success) {
                console.error(`Failed to fetch artifact URL: ${data.message}`);
            }
        } catch (error) {
            console.error('Error fetching artifact URL:', error);
        }
    }
    
    async fetchOniImage(credentials, oniImageInput) {
        try {
            
            const apiUrl = `/api/jenkins/rn-oni-image?branch=${encodeURIComponent(this.selectedBranch.name)}&username=${encodeURIComponent(credentials.username)}&token=${encodeURIComponent(credentials.token)}`;
            
            const response = await fetch(apiUrl);
            const data = await response.json();
            
            
            if (data.success && data.oni_image && oniImageInput) {
                oniImageInput.value = data.oni_image;
            }
        } catch (error) {
            console.error('Error fetching ONI image:', error);
        }
    }

    // Maps release version from customization job format to storage job format
    mapReleaseVersionToStorageFormat(releaseVersion) {
        // Handle different possible formats
        const version = String(releaseVersion).trim();
        
        // If it's already in the right format (e.g., "2503"), return as-is
        if (/^\d{4}$/.test(version)) {
            return version;
        }
        
        // If it's in format like "25.03", convert to "2503"
        if (/^\d{2}\.\d{2}$/.test(version)) {
            return version.replace('.', '');
        }
        
        // If it's in format like "24.12", convert to "2412"
        if (/^\d{2}\.\d{2}$/.test(version)) {
            return version.replace('.', '');
        }
        
        // Try to extract year and month patterns
        const match = version.match(/(\d{2})[\.\-_]?(\d{2})/);
        if (match) {
            return match[1] + match[2];
        }
        
        // If no mapping found, return null
        console.warn('Could not map release version:', version);
        return null;
    }

    initializeRNCreation() {
        // Initialize RN Creation page elements
        const branchSelector = document.getElementById('branch-selector');
        const branchSearch = document.getElementById('branch-search');
        const branchDropdownBtn = document.getElementById('branch-dropdown-btn');
        const branchDropdown = document.getElementById('branch-dropdown');
        const refreshBranchesBtn = document.getElementById('refresh-branches-btn');
        const createRNBtn = document.getElementById('create-rn-btn');
        const refreshJobBtn = document.getElementById('refresh-job-btn');
        const customizationJobUrl = document.getElementById('customization-job-url');

        if (branchSelector && branchSearch && branchDropdownBtn && branchDropdown && refreshBranchesBtn && createRNBtn) {
            // Initialize state
            this.selectedBranch = null;
            this.allBranches = [];
            this.isDropdownOpen = false;

            // Load branches on page load
            this.loadBranches();

            // Set up event listeners
            refreshBranchesBtn.addEventListener('click', () => this.loadBranches());
            branchSearch.addEventListener('click', () => this.toggleBranchDropdown());
            branchDropdownBtn.addEventListener('click', () => this.toggleBranchDropdown());

            // Add refresh job button listener
            if (refreshJobBtn) {
                refreshJobBtn.addEventListener('click', () => {
                    if (this.selectedBranch) {
                        this.loadCustomizationJob(this.selectedBranch.name);
                    }
                });
            }

            // Add customization job URL change listener
            if (customizationJobUrl) {
                customizationJobUrl.addEventListener('change', (e) => {
                    if (e.target.value) {
                        this.populateFieldsFromJob(e.target.value);
                    }
                });
            }

            // Close dropdown when clicking outside
            document.addEventListener('click', (e) => {
                if (!branchSelector.contains(e.target)) {
                    this.closeBranchDropdown();
                }
            });
        }
    }

    // RN Creation methods moved to jenkins-rn-creation.js module for better separation of concerns
}

// Global functions for HTML onclick handlers
window.sendMail = function() {
    const subject = encodeURIComponent('OCD Tool - Contact');
    window.open(`mailto:shadee.nicola@amdocs.com?subject=${subject}&body=${body}`);
};

// Initialize the application
new OCDApp();