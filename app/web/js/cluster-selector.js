// Cluster Selector Module - Multi-select combo box for EKS clusters

let allClusters = [];
let selectedClusters = new Set();
let filteredClusters = [];
let isDropdownOpen = false;
let highlightedIndex = -1; // Track highlighted option for keyboard navigation
let freeTextModeEnabled = false; // Track whether free text mode is enabled

export function initializeClusterSelector() {
    const selectorContainer = document.getElementById('cluster-selector');
    const searchInput = document.getElementById('cluster-search');
    const dropdownBtn = document.getElementById('cluster-dropdown-btn');
    const dropdown = document.getElementById('cluster-dropdown');
    const clusterList = document.getElementById('cluster-list');
    const selectedClustersContainer = document.getElementById('selected-clusters');
    const retryBtn = document.getElementById('cluster-retry-btn');

    if (!selectorContainer || !searchInput || !dropdownBtn || !dropdown) {
        console.error('Cluster selector elements not found');
        return;
    }

    // Load clusters on initialization
    loadClusters();

    // Event listeners
    selectorContainer.addEventListener('click', () => {
        if (!isDropdownOpen) {
            openDropdown();
        }
    });

    searchInput.addEventListener('input', handleSearchInput);
    searchInput.addEventListener('focus', openDropdown);
    searchInput.addEventListener('keydown', handleKeyDown);

    dropdownBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        toggleDropdown();
    });

    retryBtn.addEventListener('click', loadClusters);

    // Close dropdown when clicking outside
    document.addEventListener('click', (e) => {
        if (!selectorContainer.contains(e.target)) {
            closeDropdown();
        }
    });

    // Enable/disable scale button based on selection
    updateScaleButtonState();
}

async function loadClusters() {
    const loadingEl = document.getElementById('cluster-loading');
    const errorEl = document.getElementById('cluster-error');
    const listEl = document.getElementById('cluster-list');

    // Show loading state
    showElement(loadingEl);
    hideElement(errorEl);
    hideElement(listEl);

    try {
        const response = await fetch('/api/eks/clusters');
        const result = await response.json();

        if (result.success && result.clusters) {
            allClusters = result.clusters.sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()));
            filteredClusters = [...allClusters];
            renderClusterList();
            hideElement(loadingEl);
            showElement(listEl);
        } else {
            throw new Error(result.message || 'Failed to load clusters');
        }
    } catch (error) {
        console.error('Failed to load EKS clusters:', error);
        hideElement(loadingEl);
        showElement(errorEl);
        errorEl.querySelector('span').textContent = `Failed to load clusters: ${error.message}`;
        enableFreeTextMode(true); // Enable free text mode when cluster loading fails
    }
}

function handleSearchInput(e) {
    const searchTerm = e.target.value.toLowerCase();
    filteredClusters = allClusters.filter(cluster => 
        cluster.toLowerCase().includes(searchTerm)
    ).sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()));
    renderClusterList();
    resetHighlight(); // Reset highlight when filtering
    
    if (!isDropdownOpen) {
        openDropdown();
    }
}

function handleKeyDown(e) {
    if (e.key === 'Escape') {
        closeDropdown();
        e.target.blur();
    } else if (e.key === 'ArrowDown') {
        e.preventDefault();
        navigateOptions(1);
    } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        navigateOptions(-1);
    } else if (e.key === 'Enter') {
        e.preventDefault();
        const searchTerm = e.target.value.trim();
        const hasCustomOption = searchTerm && !filteredClusters.includes(searchTerm) && (freeTextModeEnabled || allClusters.length === 0);
        const totalOptions = filteredClusters.length + (hasCustomOption ? 1 : 0);
        
        if (highlightedIndex >= 0 && highlightedIndex < totalOptions) {
            // Select highlighted option
            if (highlightedIndex < filteredClusters.length) {
                // Regular cluster option
                const clusterName = filteredClusters[highlightedIndex];
                selectCluster(clusterName);
            } else {
                // Custom cluster option
                selectCluster(searchTerm);
            }
            e.target.value = '';
            handleSearchInput(e); // Reset filter
            resetHighlight();
        } else {
            // Try to select by search term
            if (searchTerm) {
                if (filteredClusters.includes(searchTerm)) {
                    // Cluster exists in the list
                    selectCluster(searchTerm);
                } else if (freeTextModeEnabled || allClusters.length === 0) {
                    // Free text mode: allow custom cluster names
                    selectCluster(searchTerm);
                }
                e.target.value = '';
                handleSearchInput(e); // Reset filter
            }
        }
    }
}

function toggleDropdown() {
    if (isDropdownOpen) {
        closeDropdown();
    } else {
        openDropdown();
    }
}

function openDropdown() {
    const dropdown = document.getElementById('cluster-dropdown');
    const dropdownBtn = document.getElementById('cluster-dropdown-btn');
    
    showElement(dropdown);
    dropdownBtn.classList.add('active');
    isDropdownOpen = true;
    
    // Focus search input if not already focused
    const searchInput = document.getElementById('cluster-search');
    if (document.activeElement !== searchInput) {
        searchInput.focus();
    }
}

function closeDropdown() {
    const dropdown = document.getElementById('cluster-dropdown');
    const dropdownBtn = document.getElementById('cluster-dropdown-btn');
    
    hideElement(dropdown);
    dropdownBtn.classList.remove('active');
    isDropdownOpen = false;
}

function renderClusterList() {
    const listEl = document.getElementById('cluster-list');
    const searchInput = document.getElementById('cluster-search');
    const searchTerm = searchInput ? searchInput.value.trim() : '';
    
    let listContent = '';
    
    if (filteredClusters.length === 0) {
        if (searchTerm && (freeTextModeEnabled || allClusters.length === 0)) {
            // Show option to add custom cluster name
            listContent = `
                <div class="dropdown-empty">No clusters found</div>
                <div class="cluster-option add-custom" data-cluster="${searchTerm}" data-index="0">
                    <span>Add "${searchTerm}" as custom cluster</span>
                    <span class="add-icon">+</span>
                </div>
            `;
        } else {
            listContent = '<div class="dropdown-empty">No clusters found</div>';
        }
    } else {
        listContent = filteredClusters.map((cluster, index) => {
            const isSelected = selectedClusters.has(cluster);
            const isHighlighted = index === highlightedIndex;
            return `
                <div class="cluster-option ${isSelected ? 'selected' : ''} ${isHighlighted ? 'highlighted' : ''}" data-cluster="${cluster}" data-index="${index}">
                    <span>${cluster}</span>
                    ${isSelected ? '<span class="checkmark">✓</span>' : ''}
                </div>
            `;
        }).join('');
        
        // Add custom cluster option if search term doesn't match any existing cluster
        if (searchTerm && !filteredClusters.includes(searchTerm) && (freeTextModeEnabled || allClusters.length === 0)) {
            const customIndex = filteredClusters.length;
            const isHighlighted = customIndex === highlightedIndex;
            listContent += `
                <div class="cluster-option add-custom ${isHighlighted ? 'highlighted' : ''}" data-cluster="${searchTerm}" data-index="${customIndex}">
                    <span>Add "${searchTerm}" as custom cluster</span>
                    <span class="add-icon">+</span>
                </div>
            `;
        }
    }
    
    listEl.innerHTML = listContent;

    // Add click handlers
    listEl.querySelectorAll('.cluster-option').forEach(option => {
        option.addEventListener('click', (e) => {
            e.stopPropagation();
            const clusterName = option.dataset.cluster;
            
            if (option.classList.contains('add-custom')) {
                // Add custom cluster
                selectCluster(clusterName);
                searchInput.value = '';
                handleSearchInput({ target: searchInput }); // Reset filter
            } else if (selectedClusters.has(clusterName)) {
                deselectCluster(clusterName);
            } else {
                selectCluster(clusterName);
            }
        });
    });
}

function selectCluster(clusterName) {
    if (!selectedClusters.has(clusterName)) {
        selectedClusters.add(clusterName);
        renderSelectedClusters();
        renderClusterList(); // Re-render to show checkmark
        updateScaleButtonState();
        
        // Clear search input
        const searchInput = document.getElementById('cluster-search');
        searchInput.value = '';
        filteredClusters = [...allClusters];
    }
}

function deselectCluster(clusterName) {
    selectedClusters.delete(clusterName);
    renderSelectedClusters();
    renderClusterList(); // Re-render to remove checkmark
    updateScaleButtonState();
}

function renderSelectedClusters() {
    const container = document.getElementById('selected-clusters');
    
    container.innerHTML = Array.from(selectedClusters).map(cluster => `
        <div class="selected-cluster">
            <span>${cluster}</span>
            <button type="button" class="remove-btn" data-cluster="${cluster}">×</button>
        </div>
    `).join('');

    // Add remove handlers
    container.querySelectorAll('.remove-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.stopPropagation();
            const clusterName = btn.dataset.cluster;
            deselectCluster(clusterName);
        });
    });
}

function updateScaleButtonState() {
    const scaleBtn = document.getElementById('scale-up-btn');
    if (scaleBtn) {
        scaleBtn.disabled = selectedClusters.size === 0;
    }
}

function showElement(element) {
    if (element) {
        element.style.display = 'block';
    }
}

function hideElement(element) {
    if (element) {
        element.style.display = 'none';
    }
}

// Export functions for use by other modules
export function getSelectedClusters() {
    return Array.from(selectedClusters);
}

export function clearSelectedClusters() {
    selectedClusters.clear();
    renderSelectedClusters();
    renderClusterList();
    updateScaleButtonState();
}

export function setSelectedClusters(clusters) {
    selectedClusters.clear();
    clusters.forEach(cluster => selectedClusters.add(cluster));
    renderSelectedClusters();
    renderClusterList();
    updateScaleButtonState();
}

// Keyboard navigation functions
function navigateOptions(direction) {
    if (!isDropdownOpen) {
        return;
    }

    const searchInput = document.getElementById('cluster-search');
    const searchTerm = searchInput ? searchInput.value.trim() : '';
    const hasCustomOption = searchTerm && !filteredClusters.includes(searchTerm) && (freeTextModeEnabled || allClusters.length === 0);
    const totalOptions = filteredClusters.length + (hasCustomOption ? 1 : 0);
    
    if (totalOptions === 0) {
        return;
    }

    const newIndex = highlightedIndex + direction;
    
    if (newIndex >= 0 && newIndex < totalOptions) {
        highlightedIndex = newIndex;
        renderClusterList();
        scrollToHighlighted();
    } else if (direction === 1 && highlightedIndex === -1) {
        // First arrow down - highlight first option
        highlightedIndex = 0;
        renderClusterList();
        scrollToHighlighted();
    } else if (direction === -1 && highlightedIndex === 0) {
        // Arrow up from first option - remove highlight
        highlightedIndex = -1;
        renderClusterList();
    }
}

function resetHighlight() {
    highlightedIndex = -1;
    renderClusterList();
}

function scrollToHighlighted() {
    if (highlightedIndex >= 0) {
        const listEl = document.getElementById('cluster-list');
        const highlightedEl = listEl.querySelector(`[data-index="${highlightedIndex}"]`);
        
        if (highlightedEl) {
            highlightedEl.scrollIntoView({
                block: 'nearest',
                behavior: 'smooth'
            });
        }
    }
}

function enableFreeTextMode(enabled) {
    freeTextModeEnabled = enabled;
    const searchInput = document.getElementById('cluster-search');
    
    if (enabled) {
        searchInput.placeholder = 'Type cluster name or search existing clusters...';
        // Show a note about free text mode in the error area
        const errorEl = document.getElementById('cluster-error');
        if (errorEl && errorEl.style.display !== 'none') {
            const errorSpan = errorEl.querySelector('span');
            if (errorSpan) {
                errorSpan.innerHTML = errorSpan.textContent + '<br><small>You can type a cluster name manually and press Enter to add it.</small>';
            }
        }
    } else {
        searchInput.placeholder = 'Type to search clusters...';
    }
}