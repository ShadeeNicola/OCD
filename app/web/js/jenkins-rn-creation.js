// Jenkins RN Creation Module
import { areCredentialsConfigured, showSetupModal, getSavedCredentials, getSavedBitbucketCredentials } from './settings.js';

let currentJobNumber = null;
let currentQueueURL = null;
let statusPollingInterval = null;

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
        // Initially hidden until job succeeds
        generateRNBtn.style.display = 'none';
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
            
            // Show the "Generate RN Request" button
            showGenerateRNButton();
            
            // Show Jenkins link
            if (response.job_url) {
                showJenkinsLink(response.job_url);
            }
            
            // If we have job status with queue URL, start polling
            if (response.job_status?.url?.includes('/queue/item/')) {
                currentQueueURL = response.job_status.url;
                startQueuePolling();
            } else if (response.job_status?.number) {
                currentJobNumber = response.job_status.number;
                startStatusPolling();
            }
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

function startQueuePolling() {
    if (!currentQueueURL) return;

    // Clear any existing polling
    if (statusPollingInterval) {
        clearInterval(statusPollingInterval);
    }

    // Poll every 5 seconds
    statusPollingInterval = setInterval(async () => {
        try {
            const credentials = getSavedCredentials();
            if (!credentials) {
                console.error('No credentials available for queue polling');
                return;
            }

            const response = await fetch('/api/jenkins/queue-status', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    queue_url: currentQueueURL,
                    username: credentials.username,
                    token: credentials.token
                })
            });
            const result = await response.json();

            if (result.success && result.job_status) {
                const status = result.job_status.status;
                const description = result.job_status.description;

                updateRNStatus(status, description);

                // If job has started, switch to regular polling
                if (result.job_status.number && result.job_status.url && !result.job_status.url.includes('/queue/item/')) {
                    currentJobNumber = result.job_status.number;
                    currentQueueURL = null;
                    
                    showJenkinsLink(result.job_status.url);
                    
                    clearInterval(statusPollingInterval);
                    startStatusPolling();
                    return;
                }

                // Stop polling if job failed
                if (status === 'failed') {
                    clearInterval(statusPollingInterval);
                    statusPollingInterval = null;
                    setTriggerButtonState(false);
                    showRNMessage('Storage job was cancelled or failed.', 'error');
                }
            }
        } catch (error) {
            console.error('Queue polling error:', error);
        }
    }, 5000);
}

function startStatusPolling() {
    if (!currentJobNumber) return;

    // Clear any existing polling
    if (statusPollingInterval) {
        clearInterval(statusPollingInterval);
    }

    // Poll every 10 seconds
    statusPollingInterval = setInterval(async () => {
        try {
            const credentials = getSavedCredentials();
            if (!credentials) {
                console.error('No credentials available for status polling');
                return;
            }

            const response = await fetch('/api/jenkins/status', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    job_number: currentJobNumber,
                    username: credentials.username,
                    token: credentials.token
                })
            });
            const result = await response.json();

            if (result.success && result.job_status) {
                const status = result.job_status.status;
                const description = result.job_status.description || `Job #${currentJobNumber}`;

                updateRNStatus(status, description);

                // Stop polling if job is finished
                if (status === 'success' || status === 'failed') {
                    clearInterval(statusPollingInterval);
                    statusPollingInterval = null;
                    setTriggerButtonState(false);

                    if (status === 'success') {
                        showRNMessage('Storage job completed successfully!', 'success');
                        showGenerateRNButton();
                    } else {
                        showRNMessage('Storage job failed. Check Jenkins for details.', 'error');
                    }
                }
            }
        } catch (error) {
            console.error('Status polling error:', error);
        }
    }, 10000);
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

async function copyTableToClipboard() {
    try {
        const table = document.getElementById('rn-table');
        if (!table) return;

        // Create a temporary div to hold the table content
        const tempDiv = document.createElement('div');
        tempDiv.innerHTML = table.outerHTML;
        
        // Add comprehensive styling for better formatting when pasted
        const tableHtml = `
            <style>
                table { 
                    border-collapse: collapse; 
                    width: 100%; 
                    font-size: 14px;
                    font-family: Arial, sans-serif;
                }
                th, td { 
                    border: 1px solid #ddd; 
                    padding: 12px 8px; 
                    text-align: left;
                    vertical-align: top;
                }
                th { 
                    background: linear-gradient(135deg, #f8f9fa 0%, rgba(74, 158, 255, 0.08) 100%);
                    border-bottom: 2px solid rgba(74, 158, 255, 0.2);
                    font-weight: 600;
                    color: #333;
                }
                /* Column specific widths */
                th:nth-child(1), td:nth-child(1) { width: 10%; } /* Application */
                th:nth-child(2), td:nth-child(2) { width: 15%; } /* Defect # */
                th:nth-child(3), td:nth-child(3) { width: 37%; } /* Core Patch/Charts */
                th:nth-child(4), td:nth-child(4) { width: 19%; } /* Custom Zip */
                th:nth-child(5), td:nth-child(5) { width: 8%; }  /* Commit Id */
                th:nth-child(6), td:nth-child(6) { width: 26%; } /* Comments */
                tr:nth-child(even) td { background-color: #f9f9f9; }
                tr:hover td { background-color: rgba(74, 158, 255, 0.05); }
            </style>
            ${table.outerHTML}
        `;

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
        const apiUrl = `/api/jenkins/rn-table-data?customization_job_url=${encodeURIComponent(customizationJobUrl)}&custom_orch_zip_url=${encodeURIComponent(customOrchZipUrl)}&oni_image=${encodeURIComponent(oniImage)}&username=${encodeURIComponent(credentials.username)}&token=${encodeURIComponent(credentials.token)}`;
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

        // Create HTML content with comprehensive styling
        const htmlContent = `
<!DOCTYPE html>
<html>
<head>
    <title>Release Notes Request Template</title>
    <meta charset="UTF-8">
    <style>
        body { 
            font-family: Arial, sans-serif; 
            margin: 20px; 
            background-color: #f5f5f5;
        }
        h1 { 
            color: #333; 
            text-align: center; 
            margin-bottom: 30px; 
            font-size: 24px;
        }
        .container {
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        table { 
            border-collapse: collapse; 
            width: 100%; 
            margin: 20px 0;
            font-size: 14px;
            background: white;
        }
        th, td { 
            border: 1px solid #ddd; 
            padding: 12px 8px; 
            text-align: left;
            vertical-align: top;
        }
        th { 
            background: linear-gradient(135deg, #f8f9fa 0%, rgba(74, 158, 255, 0.08) 100%);
            border-bottom: 2px solid rgba(74, 158, 255, 0.2);
            font-weight: 600;
            color: #333;
            font-size: 14px;
        }
        /* Column specific widths */
        th:nth-child(1), td:nth-child(1) { width: 10%; } /* Application */
        th:nth-child(2), td:nth-child(2) { width: 15%; } /* Defect # */
        th:nth-child(3), td:nth-child(3) { width: 37%; } /* Core Patch/Charts */
        th:nth-child(4), td:nth-child(4) { width: 19%; } /* Custom Zip */
        th:nth-child(5), td:nth-child(5) { width: 8%; }  /* Commit Id */
        th:nth-child(6), td:nth-child(6) { width: 26%; } /* Comments */
        tr:nth-child(even) td { background-color: #f9f9f9; }
        tr:hover td { background-color: rgba(74, 158, 255, 0.05); }
        @media print {
            body { background-color: white; }
            .container { box-shadow: none; }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Release Notes Request Template</h1>
        ${table.outerHTML}
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