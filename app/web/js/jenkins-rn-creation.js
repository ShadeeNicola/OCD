// Jenkins RN Creation Module
import { areCredentialsConfigured, showSetupModal, getSavedCredentials, getSavedBitbucketCredentials } from './settings.js';

let currentJobNumber = null;
let currentQueueURL = null;
let statusPollingInterval = null;
let storageJobURL = null;

export function initializeRNCreation() {
    const triggerBtn = document.getElementById('create-rn-btn');
    const generateRNBtn = document.getElementById('generate-rn-btn');

    if (triggerBtn) {
        triggerBtn.addEventListener('click', () => handleTriggerStorageJob());
        // Initially disabled until branch is selected
        triggerBtn.disabled = true;
    }

    if (generateRNBtn) {
        generateRNBtn.addEventListener('click', () => openRNTableModal());
        // Button is now always visible
    }

    // Initialize RN table modal functionality
    initializeRNTableModal();
}

async function handleTriggerStorageJob() {
    // Get the selected branch from the main app
    const selectedBranch = getSelectedBranch();
    if (!selectedBranch) {
        showRNMessage('Please select a branch first', 'error');
        return;
    }

    // Check if credentials are configured
    if (!areCredentialsConfigured()) {
        showSetupModal();
        return;
    }

    // Validate form data
    const formData = gatherFormData(selectedBranch);
    const validationResult = validateFormData(formData);
    if (!validationResult.valid) {
        showRNMessage(`Validation errors: ${validationResult.errors.join(', ')}`, 'error');
        return;
    }

    // Disable button during operation
    setTriggerButtonState(true);
    showRNStatus('queued', 'Triggering storage job...');

    try {
        const response = await triggerStorageJob(formData);
        
        if (response.success) {
            showRNMessage('Storage job triggered successfully!', 'success');
            showRNStatus('success', 'Storage job triggered successfully');
            
            // Don't show Generate RN button yet - wait for job to get build number
            
            console.log('DEBUG: Initial response job_status:', response.job_status);
            
            // Show Jenkins link but don't overwrite storageJobURL yet (wait for build number)
            if (response.job_url) {
                showJenkinsLink(response.job_url);
                // Don't set storageJobURL here as it doesn't have build number yet
                console.log('DEBUG: Received base job_url (without build number):', response.job_url);
            }
            
            // If job is queued, start polling regardless of URL format
            console.log('DEBUG: Job status:', response.job_status?.status, 'Number:', response.job_status?.number);
            // Simple approach: wait 3 seconds then get latest build number directly from Jenkins
            console.log('DEBUG: Waiting 3 seconds then fetching latest build number...');
            setTimeout(async () => {
                await getLatestBuildNumber();
            }, 3000);
        } else if (response.job_status?.number && response.job_status.number > 0) {
            currentJobNumber = response.job_status.number;
            // Set storage job URL with build number
            storageJobURL = `http://ilososp030.corp.amdocs.com:7070/job/ATT_Storage_Creation/${currentJobNumber}`;
            console.log('DEBUG: Set storageJobURL with build number from initial response:', storageJobURL);

            // Show Generate RN button since we have the build number
            showGenerateRNButton();
            // Don't start status polling - we have the build number already
        } else {
            throw new Error(response.message || 'Failed to trigger storage job');
        }

    } catch (error) {
        console.error('Storage job trigger error:', error);
        showRNMessage(`Failed to trigger storage job: ${error.message}`, 'error');
        showRNStatus('failed', 'Failed to trigger storage job');
    } finally {
        setTriggerButtonState(false);
    }
}

async function triggerStorageJob(formData) {
    const credentials = getSavedCredentials();
    
    // Add Jenkins credentials to request if available
    const requestBody = { ...formData };
    if (credentials) {
        requestBody.username = credentials.username;
        requestBody.token = credentials.token;
    }

    const response = await fetch('/api/jenkins/rn-create', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(requestBody)
    });

    const result = await response.json();

    if (!response.ok) {
        throw new Error(result.message || `HTTP ${response.status}`);
    }

    return result;
}

function gatherFormData(selectedBranch) {
    const bitbucketCredentials = getSavedBitbucketCredentials();
    
    return {
        branch: selectedBranch.name,
        product: getElementValue('product', 'oso'),
        core_version: getElementValue('core-version', ''),
        env_login: getElementValue('env-login', 'aws'),
        build_chart_version: getElementValue('build-chart-version', ''),
        branch_name: getElementValue('branch-name', selectedBranch.name),
        custom_orch_zip_url: getElementValue('custom-orch-zip-url', ''),
        oni_image: getElementValue('oni-image', ''),
        email: getElementValue('email', ''),
        layering: getElementValue('layering', 'legacy'),
        customization_job_url: getElementValue('customization-job-url', ''),
        bitbucket_username: bitbucketCredentials?.username || '',
        bitbucket_token: bitbucketCredentials?.token || ''
    };
}

function validateFormData(formData) {
    const errors = [];
    
    // Core Version is required as user must select from dropdown
    if (!formData.core_version || formData.core_version.trim() === '') {
        errors.push('Core Version is required');
    }
    
    // Email is required for notifications
    if (!formData.email || formData.email.trim() === '') {
        errors.push('Email is required');
    }
    
    // Product, env_login, and layering have sensible defaults, so no validation needed
    
    return {
        valid: errors.length === 0,
        errors: errors
    };
}

// Queue polling removed - using simple 3-second approach instead

// Status polling removed - we use simple direct Jenkins API approach instead

async function getLatestBuildNumber() {
    try {
        const credentials = getSavedCredentials();
        if (!credentials) {
            console.error('No credentials available for getting build number');
            return;
        }

        // Get the last triggered storage job parameters to match against builds
        const selectedBranch = getSelectedBranch();
        const formData = gatherFormData(selectedBranch);

        console.log('DEBUG: Frontend trigger parameters:', {
            product: formData.product,
            core_version: formData.core_version,
            branch_name: formData.branch_name,
            custom_orch_zip_url: formData.custom_orch_zip_url,
            oni_image: formData.oni_image
        });

        // Call Jenkins API with trigger parameters for matching
        const params = new URLSearchParams({
            build_url: 'http://ilososp030.corp.amdocs.com:7070/job/ATT_Storage_Creation',
            username: credentials.username,
            token: credentials.token,
            product: formData.product,
            core_version: formData.core_version,
            branch_name: formData.branch_name,
            custom_orch_zip_url: formData.custom_orch_zip_url,
            oni_image: formData.oni_image
        });

        console.log('DEBUG: API URL:', `/api/jenkins/build-info?${params.toString()}`);

        const response = await fetch(`/api/jenkins/build-info?${params.toString()}`);
        const data = await response.json();

        console.log('DEBUG: Latest build info response:', data);

        if (data.success && data.build_info && data.build_info.lastBuild && data.build_info.lastBuild.number) {
            currentJobNumber = data.build_info.lastBuild.number;
            storageJobURL = `http://ilososp030.corp.amdocs.com:7070/job/ATT_Storage_Creation/${currentJobNumber}`;
            console.log('DEBUG: Set storageJobURL from latest build:', storageJobURL);
            showGenerateRNButton();
        } else {
            console.log('DEBUG: Could not get latest build number, retrying in 2 seconds...');
            setTimeout(async () => {
                await getLatestBuildNumber();
            }, 2000);
        }
    } catch (error) {
        console.error('Error getting latest build number:', error);
    }
}

// UI Helper Functions
function showRNStatus(status, text) {
    const rnStatus = document.getElementById('rn-creation-status');
    const statusIcon = rnStatus?.querySelector('.status-icon');
    const statusText = document.getElementById('rn-creation-status-text');

    if (rnStatus && statusIcon && statusText) {
        rnStatus.style.display = 'block';
        statusIcon.className = `status-icon ${status}`;
        statusText.textContent = text;
    }
}

function updateRNStatus(status, description) {
    showRNStatus(status, description);
}

function showJenkinsLink(jobUrl) {
    const jenkinsLink = document.getElementById('jenkins-rn-job-link');
    if (jenkinsLink) {
        const link = jenkinsLink.querySelector('a');
        if (link) {
            link.href = jobUrl;
            jenkinsLink.style.display = 'block';
        }
    }
}

function showRNMessage(message, type = 'info') {
    const messageElement = document.getElementById('rn-creation-message');
    if (messageElement) {
        messageElement.textContent = message;
        messageElement.className = `status-message ${type}`;
        messageElement.style.display = 'block';

        // Auto-hide success messages after 5 seconds
        if (type === 'success') {
            setTimeout(() => {
                messageElement.style.display = 'none';
            }, 5000);
        }
    }
}

function setTriggerButtonState(disabled) {
    const triggerBtn = document.getElementById('create-rn-btn');
    
    if (triggerBtn) {
        triggerBtn.disabled = disabled;
        triggerBtn.textContent = disabled ? 'Triggering...' : 'Trigger Storage Job';
    }
}

// Utility Functions
function getElementValue(elementId, defaultValue = '') {
    const element = document.getElementById(elementId);
    return element?.value || defaultValue;
}

function getSelectedBranch() {
    // This should be provided by the main app
    // For now, we'll access it from the global scope
    return window.ocdApp?.selectedBranch || null;
}

// Enable trigger button when branch is selected
export function enableTriggerButton() {
    const triggerBtn = document.getElementById('create-rn-btn');
    if (triggerBtn) {
        triggerBtn.disabled = false;
    }
}

// RN Table Modal Functions
function initializeRNTableModal() {
    const modal = document.getElementById('rn-table-modal');
    const closeBtn = document.getElementById('close-rn-table-btn');
    const copyBtn = document.getElementById('copy-rn-table-btn');
    const downloadBtn = document.getElementById('download-rn-table-btn');

    if (closeBtn) {
        closeBtn.addEventListener('click', () => closeRNTableModal());
    }

    if (copyBtn) {
        copyBtn.addEventListener('click', () => copyTableToClipboard());
    }

    if (downloadBtn) {
        downloadBtn.addEventListener('click', () => downloadTable());
    }

    // Close modal when clicking outside
    if (modal) {
        modal.addEventListener('click', (e) => {
            if (e.target === modal) {
                closeRNTableModal();
            }
        });
    }

    // Close modal with Escape key
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            closeRNTableModal();
        }
    });
}

function showGenerateRNButton() {
    const generateRNBtn = document.getElementById('generate-rn-btn');
    if (generateRNBtn) {
        generateRNBtn.style.display = 'inline-block';
    }
}

function openRNTableModal() {
    const modal = document.getElementById('rn-table-modal');
    if (modal) {
        modal.style.display = 'flex';
        // Populate table with real data when modal opens
        populateRNTable();
    }
}

function closeRNTableModal() {
    const modal = document.getElementById('rn-table-modal');
    const successMessage = document.getElementById('copy-success-message');
    
    if (modal) {
        modal.style.display = 'none';
    }
    
    // Hide success message when closing modal
    if (successMessage) {
        successMessage.style.display = 'none';
    }
}

// createStyledTableClone creates a table clone with inline styles for email/export compatibility
function createStyledTableClone(table, includeContainer = false) {
    const tableClone = table.cloneNode(true);
    
    // Apply inline styles to headers for email compatibility
    const headers = tableClone.querySelectorAll('th');
    headers.forEach((header, index) => {
        header.style.backgroundColor = '#b3d1ff';
        header.style.borderBottom = '2px solid #2d7de8';
        header.style.border = '1px solid #ddd';
        header.style.padding = '12px 8px';
        header.style.fontWeight = '600';
        header.style.fontSize = '10px';
        header.style.fontFamily = 'Arial, sans-serif';
        header.style.color = '#333';
        header.style.textAlign = 'left';
        header.style.verticalAlign = 'top';
        
        // Apply column widths
        const widths = ['8%', '12%', '38%', '15%', '8%', '39%'];
        if (index < widths.length) {
            header.style.width = widths[index];
        }
    });
    
    // Apply inline styles to cells
    const cells = tableClone.querySelectorAll('td');
    cells.forEach((cell, index) => {
        cell.style.border = '1px solid #ddd';
        cell.style.padding = '12px 8px';
        cell.style.fontSize = '10px';
        cell.style.fontFamily = 'Arial, sans-serif';
        cell.style.textAlign = 'left';
        cell.style.verticalAlign = 'top';
        
        // Apply column widths to cells too
        const row = cell.parentElement;
        const cellIndex = Array.from(row.children).indexOf(cell);
        const widths = ['8%', '12%', '38%', '15%', '8%', '39%'];
        if (cellIndex < widths.length) {
            cell.style.width = widths[cellIndex];
        }
        
        // Add alternating row colors for download version
        if (includeContainer) {
            const rowIndex = Array.from(row.parentElement.children).indexOf(row);
            if (rowIndex % 2 === 1) { // Skip header row (index 0)
                cell.style.backgroundColor = '#f9f9f9';
            }
        }
    });
    
    // Apply table styles
    tableClone.style.borderCollapse = 'collapse';
    tableClone.style.width = '100%';
    tableClone.style.fontSize = '10px';
    tableClone.style.fontFamily = 'Arial, sans-serif';
    
    if (includeContainer) {
        tableClone.style.margin = '20px 0';
    }
    
    return tableClone;
}

async function copyTableToClipboard() {
    try {
        const table = document.getElementById('rn-table');
        if (!table) return;

        // Create styled table clone for clipboard
        const tableClone = createStyledTableClone(table, false);
        const tableHtml = tableClone.outerHTML;

        // Try to use the modern Clipboard API first
        if (navigator.clipboard && navigator.clipboard.write) {
            const blob = new Blob([tableHtml], { type: 'text/html' });
            const data = [new ClipboardItem({ 'text/html': blob })];
            await navigator.clipboard.write(data);
        } else if (navigator.clipboard && navigator.clipboard.writeText) {
            // Fallback to plain text
            const tableText = getTableAsText(table);
            await navigator.clipboard.writeText(tableText);
        } else {
            // Fallback for older browsers
            const textArea = document.createElement('textarea');
            textArea.value = getTableAsText(table);
            document.body.appendChild(textArea);
            textArea.select();
            document.execCommand('copy');
            document.body.removeChild(textArea);
        }

        showCopySuccess();

    } catch (error) {
        console.error('Failed to copy table to clipboard:', error);
        // Show error message to user
        showRNMessage('Failed to copy table to clipboard', 'error');
    }
}

function getTableAsText(table) {
    let text = '';
    const rows = table.querySelectorAll('tr');
    
    rows.forEach(row => {
        const cells = row.querySelectorAll('th, td');
        const rowData = Array.from(cells).map(cell => cell.textContent.trim()).join('\t');
        text += rowData + '\n';
    });
    
    return text;
}

function showCopySuccess() {
    const successMessage = document.getElementById('copy-success-message');
    if (successMessage) {
        successMessage.style.display = 'block';
        
        // Hide the message after 3 seconds
        setTimeout(() => {
            successMessage.style.display = 'none';
        }, 3000);
    }
}

async function populateRNTable() {
    try {
        // Reset storage job URL to force fresh parameter matching search
        console.log('DEBUG: Resetting storageJobURL and currentJobNumber for fresh search');
        storageJobURL = null;
        currentJobNumber = null;

        // Show loading indication
        updateRNTableContent({
            application: 'NEO-OSO',
            defect_number: '[To be populated]',
            core_patch_charts: 'Loading data from cluster...',
            custom_orchestration_zip: 'Loading artifact URL...',
            commit_id: 'NA',
            comments_instructions: 'Fetching TLC version, cluster name, and image versions...'
        });

        // Get current form data to determine what to populate
        const customizationJobUrl = getElementValue('customization-job-url', '');
        const customOrchZipUrl = getElementValue('custom-orch-zip-url', '');
        const oniImage = getElementValue('oni-image', '');

        console.log('DEBUG: populateRNTable called');
        console.log('DEBUG: customizationJobUrl:', customizationJobUrl);
        console.log('DEBUG: customOrchZipUrl:', customOrchZipUrl);

        if (!customizationJobUrl) {
            console.warn('ERROR: No customization job URL available for RN table population');
            // Show placeholder data with indication that job URL is missing
            updateRNTableContent({
                application: 'NEO-OSO',
                defect_number: '[To be populated]',
                core_patch_charts: '[No customization job URL - please select branch first]',
                custom_orchestration_zip: customOrchZipUrl ? `Custom Orchestration ZIP: ${customOrchZipUrl}` : '[To be populated]',
                commit_id: 'NA',
                comments_instructions: '[To be populated]'
            });
            return;
        }

        // Get credentials (same pattern as fetchBuildParameters)
        const credentials = getSavedCredentials();
        if (!credentials) {
            console.error('ERROR: No credentials available for RN table data fetching');
            updateRNTableContent({
                application: 'NEO-OSO',
                defect_number: '[To be populated]',
                core_patch_charts: '[No credentials - please configure Jenkins credentials in Settings]',
                custom_orchestration_zip: customOrchZipUrl ? `Custom Orchestration ZIP: ${customOrchZipUrl}` : '[To be populated]',
                commit_id: 'NA',
                comments_instructions: '[To be populated]'
            });
            return;
        }

        // Fetch RN table data from backend with credentials (same pattern as fetchBuildParameters)
        console.log('DEBUG: storageJobURL before API call:', storageJobURL);
        console.log('DEBUG: currentJobNumber:', currentJobNumber);

        // If no storage job URL, automatically find latest matching storage build
        if (!storageJobURL || !currentJobNumber) {
            console.log('DEBUG: No storage job URL found, running automatic parameter matching...');
            await getLatestBuildNumber();
        }

        // Ensure we have the build number in the URL
        if (storageJobURL && currentJobNumber && !storageJobURL.includes(currentJobNumber.toString())) {
            storageJobURL = `http://ilososp030.corp.amdocs.com:7070/job/ATT_Storage_Creation/${currentJobNumber}`;
            console.log('DEBUG: Fixed storageJobURL with build number:', storageJobURL);
        }

        const apiUrl = `/api/jenkins/rn-table-data?customization_job_url=${encodeURIComponent(customizationJobUrl)}&custom_orch_zip_url=${encodeURIComponent(customOrchZipUrl)}&oni_image=${encodeURIComponent(oniImage)}&storage_job_url=${encodeURIComponent(storageJobURL || '')}&username=${encodeURIComponent(credentials.username)}&token=${encodeURIComponent(credentials.token)}`;
        console.log('DEBUG: Calling API URL:', apiUrl);
        
        const response = await fetch(apiUrl);
        console.log('DEBUG: API response status:', response.status);
        
        const data = await response.json();
        console.log('DEBUG: API response data:', data);

        if (data.success && data.rn_table_data) {
            console.log('DEBUG: Updating table with data:', data.rn_table_data);
            updateRNTableContent(data.rn_table_data);
        } else {
            console.error('ERROR: Failed to fetch RN table data:', data.message);
            // Show error in the table
            updateRNTableContent({
                application: 'NEO-OSO',
                defect_number: '[To be populated]',
                core_patch_charts: `[API Error: ${data.message || 'Unknown error'}]`,
                custom_orchestration_zip: customOrchZipUrl ? `Custom Orchestration ZIP: ${customOrchZipUrl}` : '[To be populated]',
                commit_id: 'NA',
                comments_instructions: '[To be populated]'
            });
        }

    } catch (error) {
        console.error('ERROR: Exception in populateRNTable:', error);
        // Show error in the table
        updateRNTableContent({
            application: 'NEO-OSO',
            defect_number: '[To be populated]',
            core_patch_charts: `[Exception: ${error.message || 'Unknown error'}]`,
            custom_orchestration_zip: '[To be populated]',
            commit_id: 'NA',
            comments_instructions: '[To be populated]'
        });
    }
}

function updateRNTableContent(rnTableData) {
    const tableBody = document.getElementById('rn-table-body');
    if (!tableBody) return;

    // Clear existing content
    tableBody.innerHTML = '';

    // Create new row with real data
    const row = document.createElement('tr');
    row.innerHTML = `
        <td>${escapeHtml(rnTableData.application || 'NEO-OSO')}</td>
        <td>${escapeHtml(rnTableData.defect_number || '[To be populated]')}</td>
        <td>${formatWithLineBreaks(rnTableData.core_patch_charts || '[To be populated]')}</td>
        <td>${escapeHtml(rnTableData.custom_orchestration_zip || '[To be populated]')}</td>
        <td>${escapeHtml(rnTableData.commit_id || 'NA')}</td>
        <td>${formatWithLineBreaks(rnTableData.comments_instructions || '[To be populated]')}</td>
    `;

    tableBody.appendChild(row);
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function formatWithLineBreaks(text) {
    // First escape HTML to prevent XSS
    const escapedText = escapeHtml(text);
    // Then convert newlines to <br> tags
    return escapedText.replace(/\n/g, '<br>');
}

function downloadTable() {
    try {
        const table = document.getElementById('rn-table');
        if (!table) return;

        // Create styled table clone for download (with alternating rows)
        const tableClone = createStyledTableClone(table, true);
        
        // Create simple HTML content with inline styles
        const htmlContent = `
<!DOCTYPE html>
<html>
<head>
    <title>Release Notes Request Template</title>
    <meta charset="UTF-8">
</head>
<body style="font-family: Arial, sans-serif; margin: 20px; background-color: white;">
    <div style="background: white; padding: 20px;">
        <h1 style="color: #333; text-align: center; margin-bottom: 30px; font-size: 24px; font-family: Arial, sans-serif;">Release Notes Request Template</h1>
        ${tableClone.outerHTML}
    </div>
</body>
</html>`;

        // Create and download the file
        const blob = new Blob([htmlContent], { type: 'text/html' });
        const url = URL.createObjectURL(blob);
        
        const link = document.createElement('a');
        link.href = url;
        link.download = 'release-notes-template.html';
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        
        // Clean up the URL object
        URL.revokeObjectURL(url);

        // Show success message
        showRNMessage('Release notes template downloaded successfully!', 'success');

    } catch (error) {
        console.error('Failed to download table:', error);
        showRNMessage('Failed to download table', 'error');
    }
}

// Clean up polling when page unloads
window.addEventListener('beforeunload', () => {
    if (statusPollingInterval) {
        clearInterval(statusPollingInterval);
    }
});