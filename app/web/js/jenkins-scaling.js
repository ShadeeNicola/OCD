// Jenkins Scaling Module
import { areCredentialsConfigured, showSetupModal, getSavedCredentials } from './settings.js';
import { getSelectedClusters, clearSelectedClusters } from './cluster-selector.js';

let currentJobNumber = null;
let currentQueueURL = null;
let statusPollingInterval = null;

export function initializeScaling() {
    const scaleUpBtn = document.getElementById('scale-up-btn');

    if (scaleUpBtn) {
        scaleUpBtn.addEventListener('click', () => handleScaleAction('up'));
        // Initially disabled until clusters are selected
        scaleUpBtn.disabled = true;
    }
}

async function handleScaleAction(scaleType) {
    const selectedClusters = getSelectedClusters();

    if (selectedClusters.length === 0) {
        showScalingMessage('Please select at least one cluster', 'error');
        return;
    }

    // Check if credentials are configured
    if (!areCredentialsConfigured()) {
        showSetupModal();
        return;
    }

    // Disable buttons during scaling
    setScalingButtonsState(true);
    
    const clusterCount = selectedClusters.length;
    const clusterText = clusterCount === 1 ? selectedClusters[0] : `${clusterCount} clusters`;
    showScalingStatus('queued', `Triggering scale up for ${clusterText}...`);

    try {
        const credentials = getSavedCredentials();
        const scalingPromises = selectedClusters.map(clusterName => 
            triggerClusterScale(clusterName, scaleType, credentials)
        );

        const results = await Promise.allSettled(scalingPromises);
        
        const successful = results.filter(r => r.status === 'fulfilled' && r.value.success);
        const failed = results.filter(r => r.status === 'rejected' || !r.value.success);

        if (successful.length === selectedClusters.length) {
            showScalingMessage(`Successfully triggered scale up for all ${clusterCount} clusters!`, 'success');
            showScalingStatus('success', `Scale up triggered for all clusters`);
        } else if (successful.length > 0) {
            showScalingMessage(`Scale up triggered for ${successful.length}/${clusterCount} clusters`, 'warning');
            showScalingStatus('warning', `Partial success: ${successful.length}/${clusterCount} clusters`);
        } else {
            throw new Error('Failed to trigger scaling for any clusters');
        }

        // Show the first successful Jenkins link (if any)
        const firstSuccess = successful.find(r => r.value.job_status?.url);
        if (firstSuccess) {
            showJenkinsLink(firstSuccess.value.job_status.url);
        }

        // Clear selection after successful scaling
        if (successful.length > 0) {
            setTimeout(() => {
                clearSelectedClusters();
            }, 2000);
        }

    } catch (error) {
        console.error('Scaling error:', error);
        showScalingMessage(`Failed to trigger scale up: ${error.message}`, 'error');
        showScalingStatus('failed', 'Failed to start scale up');
    } finally {
        setScalingButtonsState(false);
    }
}

async function triggerClusterScale(clusterName, scaleType, credentials) {
    const requestBody = {
        cluster_name: clusterName,
        scale_type: scaleType,
        account: 'ATT'
    };

    // Add credentials to request if available
    if (credentials) {
        requestBody.username = credentials.username;
        requestBody.token = credentials.token;
    }

    const response = await fetch('/api/jenkins/scale', {
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

function startQueuePolling() {
    if (!currentQueueURL) return;

    // Clear any existing polling
    if (statusPollingInterval) {
        clearInterval(statusPollingInterval);
    }

    // Poll every 5 seconds (more frequent for queue items)
    statusPollingInterval = setInterval(async () => {
        try {
            const credentials = getSavedCredentials();
            if (!credentials) {
                console.error('No credentials available for queue polling');
                return;
            }

            const response = await fetch('/api/jenkins/queue', {
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

                updateScalingStatus(status, description);

                // If job has started (got a job number), switch to regular polling
                if (result.job_status.number && result.job_status.url && !result.job_status.url.includes('/queue/item/')) {
                    currentJobNumber = result.job_status.number;
                    currentQueueURL = null;
                    
                    // Update the Jenkins link to the actual job
                    showJenkinsLink(result.job_status.url);
                    
                    // Clear queue polling and start job polling
                    clearInterval(statusPollingInterval);
                    startStatusPolling();
                    return;
                }

                // Stop polling if job is cancelled or failed
                if (status === 'failed') {
                    clearInterval(statusPollingInterval);
                    statusPollingInterval = null;
                    setScalingButtonsState(false);
                    showScalingMessage('Environment scaling was cancelled or failed.', 'error');
                }
            }
        } catch (error) {
            console.error('Queue polling error:', error);
            // Don't stop polling on single errors, but log them
        }
    }, 5000); // Poll every 5 seconds
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

                updateScalingStatus(status, description);

                // Stop polling if job is finished
                if (status === 'success' || status === 'failed') {
                    clearInterval(statusPollingInterval);
                    statusPollingInterval = null;
                    setScalingButtonsState(false);

                    if (status === 'success') {
                        showScalingMessage('Environment scaling completed successfully!', 'success');
                    } else {
                        showScalingMessage('Environment scaling failed. Check Jenkins for details.', 'error');
                    }
                }
            }
        } catch (error) {
            console.error('Status polling error:', error);
            // Don't stop polling on single errors, but log them
        }
    }, 10000); // Poll every 10 seconds
}

function showScalingStatus(status, text) {
    const scalingStatus = document.getElementById('scaling-status');
    const statusIcon = document.querySelector('.status-icon');
    const statusText = document.getElementById('scaling-status-text');

    if (scalingStatus && statusIcon && statusText) {
        scalingStatus.style.display = 'block';
        statusIcon.className = `status-icon ${status}`;
        statusText.textContent = text;
    }
}

function updateScalingStatus(status, description) {
    showScalingStatus(status, description);
}

function showJenkinsLink(jobUrl) {
    const jenkinsLink = document.getElementById('jenkins-job-link');
    if (jenkinsLink) {
        const link = jenkinsLink.querySelector('a');
        if (link) {
            link.href = jobUrl;
            jenkinsLink.style.display = 'block';
        }
    }
}

function showScalingMessage(message, type = 'info') {
    const messageElement = document.getElementById('scaling-message');
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

function setScalingButtonsState(disabled) {
    const scaleUpBtn = document.getElementById('scale-up-btn');
    
    if (scaleUpBtn) scaleUpBtn.disabled = disabled;
}

// Clean up polling when page unloads
window.addEventListener('beforeunload', () => {
    if (statusPollingInterval) {
        clearInterval(statusPollingInterval);
    }
});