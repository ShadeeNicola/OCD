// Settings Management Module
const STORAGE_KEY = 'ocd_jenkins_credentials';
const BITBUCKET_STORAGE_KEY = 'ocd_bitbucket_credentials';

export function initializeSettings() {
    // Jenkins form elements
    const usernameInput = document.getElementById('jenkins-username');
    const tokenInput = document.getElementById('jenkins-token');
    const toggleTokenBtn = document.getElementById('toggle-token-btn');
    const saveBtn = document.getElementById('save-settings-btn');
    const testBtn = document.getElementById('test-connection-btn');
    const clearBtn = document.getElementById('clear-settings-btn');

    // Bitbucket form elements
    const bitbucketUsernameInput = document.getElementById('bitbucket-username');
    const bitbucketTokenInput = document.getElementById('bitbucket-token');
    const toggleBitbucketTokenBtn = document.getElementById('toggle-bitbucket-token-btn');
    const saveBitbucketBtn = document.getElementById('save-bitbucket-settings-btn');
    const testBitbucketBtn = document.getElementById('test-bitbucket-connection-btn');
    const clearBitbucketBtn = document.getElementById('clear-bitbucket-settings-btn');

    // Modal elements
    const modal = document.getElementById('jenkins-setup-modal');
    const closeModalBtn = document.getElementById('close-modal-btn');
    const gotoSettingsBtn = document.getElementById('goto-settings-btn');
    const cancelModalBtn = document.getElementById('cancel-modal-btn');

    // Load existing credentials on page load
    loadSavedCredentials();
    loadSavedBitbucketCredentials();

    // Jenkins event listeners
    if (toggleTokenBtn) {
        toggleTokenBtn.addEventListener('click', togglePasswordVisibility);
    }

    if (saveBtn) {
        saveBtn.addEventListener('click', saveCredentials);
    }

    if (testBtn) {
        testBtn.addEventListener('click', testConnection);
    }

    if (clearBtn) {
        clearBtn.addEventListener('click', clearCredentials);
    }

    // Bitbucket event listeners
    if (toggleBitbucketTokenBtn) {
        toggleBitbucketTokenBtn.addEventListener('click', toggleBitbucketPasswordVisibility);
    }

    if (saveBitbucketBtn) {
        saveBitbucketBtn.addEventListener('click', saveBitbucketCredentials);
    }

    if (testBitbucketBtn) {
        testBitbucketBtn.addEventListener('click', testBitbucketConnection);
    }

    if (clearBitbucketBtn) {
        clearBitbucketBtn.addEventListener('click', clearBitbucketCredentials);
    }

    // Modal event listeners
    if (closeModalBtn) {
        closeModalBtn.addEventListener('click', hideModal);
    }

    if (gotoSettingsBtn) {
        gotoSettingsBtn.addEventListener('click', () => {
            hideModal();
            navigateToSettings();
        });
    }

    if (cancelModalBtn) {
        cancelModalBtn.addEventListener('click', hideModal);
    }

    // Close modal on overlay click
    if (modal) {
        modal.addEventListener('click', (e) => {
            if (e.target === modal) {
                hideModal();
            }
        });
    }

    // Enable/disable buttons based on input
    [usernameInput, tokenInput, bitbucketUsernameInput, bitbucketTokenInput].forEach(input => {
        if (input) {
            input.addEventListener('input', updateButtonStates);
        }
    });

    updateButtonStates();
}

function loadSavedCredentials() {
    try {
        const saved = localStorage.getItem(STORAGE_KEY);
        if (saved) {
            const credentials = JSON.parse(saved);
            const usernameInput = document.getElementById('jenkins-username');
            const tokenInput = document.getElementById('jenkins-token');
            
            if (usernameInput && credentials.username) {
                usernameInput.value = credentials.username;
            }
            if (tokenInput && credentials.token) {
                tokenInput.value = credentials.token;
            }
            
            updateButtonStates();
        }
    } catch (error) {
        console.error('Error loading saved credentials:', error);
    }
}

function saveCredentials() {
    const usernameInput = document.getElementById('jenkins-username');
    const tokenInput = document.getElementById('jenkins-token');
    
    const username = usernameInput?.value.trim();
    const token = tokenInput?.value.trim();

    if (!username || !token) {
        showSettingsMessage('Please enter both username and token', 'error');
        return;
    }

    try {
        const credentials = { username, token };
        localStorage.setItem(STORAGE_KEY, JSON.stringify(credentials));
        showSettingsMessage('Credentials saved successfully!', 'success');
        updateButtonStates();
    } catch (error) {
        console.error('Error saving credentials:', error);
        showSettingsMessage('Failed to save credentials', 'error');
    }
}

async function testConnection() {
    const credentials = getSavedCredentials();
    if (!credentials) {
        showSettingsMessage('Please save credentials first', 'error');
        return;
    }

    showSettingsStatus('running', 'Testing connection...');
    
    try {
        // Test by making a simple API call to Jenkins
        const response = await fetch('/api/jenkins/scale', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                cluster_name: 'test-connection',
                scale_type: 'up',
                account: 'ATT',
                username: credentials.username,
                token: credentials.token
            })
        });

        const result = await response.json();

        if (response.ok && result.success) {
            showSettingsStatus('success', 'Connection successful!');
            showSettingsMessage('Jenkins connection verified!', 'success');
        } else {
            throw new Error(result.message || 'Connection failed');
        }
    } catch (error) {
        console.error('Connection test failed:', error);
        showSettingsStatus('failed', 'Connection failed');
        showSettingsMessage(`Connection test failed: ${error.message}`, 'error');
    }
}

function clearCredentials() {
    if (confirm('Are you sure you want to clear your Jenkins credentials?')) {
        try {
            localStorage.removeItem(STORAGE_KEY);
            const usernameInput = document.getElementById('jenkins-username');
            const tokenInput = document.getElementById('jenkins-token');
            
            if (usernameInput) usernameInput.value = '';
            if (tokenInput) tokenInput.value = '';
            
            hideSettingsStatus();
            showSettingsMessage('Credentials cleared', 'success');
            updateButtonStates();
        } catch (error) {
            console.error('Error clearing credentials:', error);
            showSettingsMessage('Failed to clear credentials', 'error');
        }
    }
}

function togglePasswordVisibility() {
    const tokenInput = document.getElementById('jenkins-token');
    const eyeIcon = document.getElementById('eye-icon');
    
    if (tokenInput && eyeIcon) {
        const isPassword = tokenInput.type === 'password';
        tokenInput.type = isPassword ? 'text' : 'password';
        
        // Update eye icon
        eyeIcon.setAttribute('d', isPassword 
            ? 'M12 4.5C7 4.5 2.73 7.61 1 12c1.73 4.39 6 7.5 11 7.5s9.27-3.11 11-7.5c-1.73-4.39-6-7.5-11-7.5zM12 17c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5zm0-8c-1.66 0-3 1.34-3 3s1.34 3 3 3 3-1.34 3-3-1.34-3-3-3z'
            : 'M12 7c2.76 0 5 2.24 5 5 0 .65-.13 1.26-.36 1.83l2.92 2.92c1.51-1.26 2.7-2.89 3.43-4.75-1.73-4.39-6-7.5-11-7.5-1.4 0-2.74.25-3.98.7l2.16 2.16C10.74 7.13 11.35 7 12 7zM2 4.27l2.28 2.28.46.46C3.08 8.3 1.78 10.02 1 12c1.73 4.39 6 7.5 11 7.5 1.55 0 3.03-.3 4.38-.84l.42.42L19.73 22 21 20.73 3.27 3 2 4.27zM7.53 9.8l1.55 1.55c-.05.21-.08.43-.08.65 0 1.66 1.34 3 3 3 .22 0 .44-.03.65-.08l1.55 1.55c-.67.33-1.41.53-2.2.53-2.76 0-5-2.24-5-5 0-.79.2-1.53.53-2.2zm4.31-.78l3.15 3.15.02-.16c0-1.66-1.34-3-3-3l-.17.01z'
        );
    }
}

function updateButtonStates() {
    // Jenkins buttons
    const usernameInput = document.getElementById('jenkins-username');
    const tokenInput = document.getElementById('jenkins-token');
    const saveBtn = document.getElementById('save-settings-btn');
    const testBtn = document.getElementById('test-connection-btn');
    const clearBtn = document.getElementById('clear-settings-btn');

    const hasUsername = usernameInput?.value.trim();
    const hasToken = tokenInput?.value.trim();
    const hasBoth = hasUsername && hasToken;
    const hasSaved = !!getSavedCredentials();

    if (saveBtn) saveBtn.disabled = !hasBoth;
    if (testBtn) testBtn.disabled = !hasSaved;
    if (clearBtn) clearBtn.disabled = !hasSaved;

    // Bitbucket buttons
    const bitbucketUsernameInput = document.getElementById('bitbucket-username');
    const bitbucketTokenInput = document.getElementById('bitbucket-token');
    const saveBitbucketBtn = document.getElementById('save-bitbucket-settings-btn');
    const testBitbucketBtn = document.getElementById('test-bitbucket-connection-btn');
    const clearBitbucketBtn = document.getElementById('clear-bitbucket-settings-btn');

    const hasBitbucketUsername = bitbucketUsernameInput?.value.trim();
    const hasBitbucketToken = bitbucketTokenInput?.value.trim();
    const hasBitbucketBoth = hasBitbucketUsername && hasBitbucketToken;
    const hasBitbucketSaved = !!getSavedBitbucketCredentials();

    if (saveBitbucketBtn) saveBitbucketBtn.disabled = !hasBitbucketBoth;
    if (testBitbucketBtn) testBitbucketBtn.disabled = !hasBitbucketSaved;
    if (clearBitbucketBtn) clearBitbucketBtn.disabled = !hasBitbucketSaved;
}

function showSettingsStatus(status, text) {
    const settingsStatus = document.getElementById('settings-status');
    const statusIcon = settingsStatus?.querySelector('.status-icon');
    const statusText = document.getElementById('settings-status-text');

    if (settingsStatus && statusIcon && statusText) {
        settingsStatus.style.display = 'block';
        statusIcon.className = `status-icon ${status}`;
        statusText.textContent = text;
    }
}

function hideSettingsStatus() {
    const settingsStatus = document.getElementById('settings-status');
    if (settingsStatus) {
        settingsStatus.style.display = 'none';
    }
}

function showSettingsMessage(message, type = 'info') {
    const messageElement = document.getElementById('settings-message');
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

function showModal() {
    const modal = document.getElementById('jenkins-setup-modal');
    if (modal) {
        modal.style.display = 'flex';
        // Prevent body scroll when modal is open
        document.body.style.overflow = 'hidden';
    }
}

function hideModal() {
    const modal = document.getElementById('jenkins-setup-modal');
    if (modal) {
        modal.style.display = 'none';
        // Restore body scroll
        document.body.style.overflow = '';
    }
}

function navigateToSettings() {
    // Navigate to settings page using the main app's navigation
    const settingsLink = document.querySelector('[data-page="settings"]');
    if (settingsLink) {
        settingsLink.click();
        
        // Focus on username input after page transition
        setTimeout(() => {
            const usernameInput = document.getElementById('jenkins-username');
            if (usernameInput) {
                usernameInput.focus();
            }
        }, 350);
    }
}

// Public function to get saved credentials
export function getSavedCredentials() {
    try {
        const saved = localStorage.getItem(STORAGE_KEY);
        return saved ? JSON.parse(saved) : null;
    } catch (error) {
        console.error('Error getting saved credentials:', error);
        return null;
    }
}

// Public function to show the setup modal
export function showSetupModal() {
    showModal();
}

// Public function to check if credentials are configured
export function areCredentialsConfigured() {
    const credentials = getSavedCredentials();
    return !!(credentials && credentials.username && credentials.token);
}

// Bitbucket credentials functions
function loadSavedBitbucketCredentials() {
    try {
        const saved = localStorage.getItem(BITBUCKET_STORAGE_KEY);
        if (saved) {
            const credentials = JSON.parse(saved);
            const usernameInput = document.getElementById('bitbucket-username');
            const tokenInput = document.getElementById('bitbucket-token');
            
            if (usernameInput && credentials.username) {
                usernameInput.value = credentials.username;
            }
            if (tokenInput && credentials.token) {
                tokenInput.value = credentials.token;
            }
            
            updateButtonStates();
        }
    } catch (error) {
        console.error('Error loading saved Bitbucket credentials:', error);
    }
}

function saveBitbucketCredentials() {
    const usernameInput = document.getElementById('bitbucket-username');
    const tokenInput = document.getElementById('bitbucket-token');
    
    const username = usernameInput?.value.trim();
    const token = tokenInput?.value.trim();

    if (!username || !token) {
        showBitbucketSettingsMessage('Please enter both username and token', 'error');
        return;
    }

    try {
        const credentials = { username, token };
        localStorage.setItem(BITBUCKET_STORAGE_KEY, JSON.stringify(credentials));
        showBitbucketSettingsMessage('Bitbucket credentials saved successfully!', 'success');
        updateButtonStates();
    } catch (error) {
        console.error('Error saving Bitbucket credentials:', error);
        showBitbucketSettingsMessage('Failed to save Bitbucket credentials', 'error');
    }
}

async function testBitbucketConnection() {
    const credentials = getSavedBitbucketCredentials();
    if (!credentials) {
        showBitbucketSettingsMessage('Please save Bitbucket credentials first', 'error');
        return;
    }

    showBitbucketSettingsStatus('running', 'Testing Bitbucket connection...');
    
    try {
        const response = await fetch('/api/git/branches/customization', {
            method: 'GET',
            headers: {
                'X-Bitbucket-Username': credentials.username,
                'X-Bitbucket-Token': credentials.token
            }
        });

        const result = await response.json();

        if (response.ok && result.success) {
            showBitbucketSettingsStatus('success', 'Bitbucket connection successful!');
            showBitbucketSettingsMessage(`Bitbucket connection verified! Found ${result.branches?.length || 0} branches.`, 'success');
        } else {
            throw new Error(result.message || 'Connection failed');
        }
    } catch (error) {
        console.error('Bitbucket connection test failed:', error);
        showBitbucketSettingsStatus('failed', 'Bitbucket connection failed');
        showBitbucketSettingsMessage(`Bitbucket connection test failed: ${error.message}`, 'error');
    }
}

function clearBitbucketCredentials() {
    if (confirm('Are you sure you want to clear your Bitbucket credentials?')) {
        try {
            localStorage.removeItem(BITBUCKET_STORAGE_KEY);
            const usernameInput = document.getElementById('bitbucket-username');
            const tokenInput = document.getElementById('bitbucket-token');
            
            if (usernameInput) usernameInput.value = '';
            if (tokenInput) tokenInput.value = '';
            
            hideBitbucketSettingsStatus();
            showBitbucketSettingsMessage('Bitbucket credentials cleared', 'success');
            updateButtonStates();
        } catch (error) {
            console.error('Error clearing Bitbucket credentials:', error);
            showBitbucketSettingsMessage('Failed to clear Bitbucket credentials', 'error');
        }
    }
}

function toggleBitbucketPasswordVisibility() {
    const tokenInput = document.getElementById('bitbucket-token');
    const eyeIcon = document.getElementById('bitbucket-eye-icon');
    
    if (tokenInput && eyeIcon) {
        const isPassword = tokenInput.type === 'password';
        tokenInput.type = isPassword ? 'text' : 'password';
        
        // Update eye icon
        eyeIcon.setAttribute('d', isPassword 
            ? 'M12 4.5C7 4.5 2.73 7.61 1 12c1.73 4.39 6 7.5 11 7.5s9.27-3.11 11-7.5c-1.73-4.39-6-7.5-11-7.5zM12 17c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5zm0-8c-1.66 0-3 1.34-3 3s1.34 3 3 3 3-1.34 3-3-1.34-3-3-3z'
            : 'M12 7c2.76 0 5 2.24 5 5 0 .65-.13 1.26-.36 1.83l2.92 2.92c1.51-1.26 2.7-2.89 3.43-4.75-1.73-4.39-6-7.5-11-7.5-1.4 0-2.74.25-3.98.7l2.16 2.16C10.74 7.13 11.35 7 12 7zM2 4.27l2.28 2.28.46.46C3.08 8.3 1.78 10.02 1 12c1.73 4.39 6 7.5 11 7.5 1.55 0 3.03-.3 4.38-.84l.42.42L19.73 22 21 20.73 3.27 3 2 4.27zM7.53 9.8l1.55 1.55c-.05.21-.08.43-.08.65 0 1.66 1.34 3 3 3 .22 0 .44-.03.65-.08l1.55 1.55c-.67.33-1.41.53-2.2.53-2.76 0-5-2.24-5-5 0-.79.2-1.53.53-2.2zm4.31-.78l3.15 3.15.02-.16c0-1.66-1.34-3-3-3l-.17.01z'
        );
    }
}

function showBitbucketSettingsStatus(status, text) {
    const settingsStatus = document.getElementById('bitbucket-settings-status');
    const statusIcon = settingsStatus?.querySelector('.status-icon');
    const statusText = document.getElementById('bitbucket-settings-status-text');

    if (settingsStatus && statusIcon && statusText) {
        settingsStatus.style.display = 'block';
        statusIcon.className = `status-icon ${status}`;
        statusText.textContent = text;
    }
}

function hideBitbucketSettingsStatus() {
    const settingsStatus = document.getElementById('bitbucket-settings-status');
    if (settingsStatus) {
        settingsStatus.style.display = 'none';
    }
}

function showBitbucketSettingsMessage(message, type = 'info') {
    const messageElement = document.getElementById('bitbucket-settings-message');
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

// Public function to get saved Bitbucket credentials
export function getSavedBitbucketCredentials() {
    try {
        const saved = localStorage.getItem(BITBUCKET_STORAGE_KEY);
        return saved ? JSON.parse(saved) : null;
    } catch (error) {
        console.error('Error getting saved Bitbucket credentials:', error);
        return null;
    }
}

// Public function to check if Bitbucket credentials are configured
export function areBitbucketCredentialsConfigured() {
    const credentials = getSavedBitbucketCredentials();
    return !!(credentials && credentials.username && credentials.token);
}