// Cluster Selector Module - Multi-select combo box for EKS clusters

let allClusters = [];
let selectedClusters = new Set();
let filteredClusters = [];
let isDropdownOpen = false;

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
            allClusters = result.clusters.sort();
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
    }
}

function handleSearchInput(e) {
    const searchTerm = e.target.value.toLowerCase();
    filteredClusters = allClusters.filter(cluster => 
        cluster.toLowerCase().includes(searchTerm)
    );
    renderClusterList();
    
    if (!isDropdownOpen) {
        openDropdown();
    }
}

function handleKeyDown(e) {
    if (e.key === 'Escape') {
        closeDropdown();
        e.target.blur();
    } else if (e.key === 'Enter') {
        e.preventDefault();
        const searchTerm = e.target.value.trim();
        if (searchTerm && filteredClusters.includes(searchTerm)) {
            selectCluster(searchTerm);
            e.target.value = '';
            handleSearchInput(e); // Reset filter
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
    
    if (filteredClusters.length === 0) {
        listEl.innerHTML = '<div class="dropdown-empty">No clusters found</div>';
        return;
    }

    listEl.innerHTML = filteredClusters.map(cluster => {
        const isSelected = selectedClusters.has(cluster);
        return `
            <div class="cluster-option ${isSelected ? 'selected' : ''}" data-cluster="${cluster}">
                <span>${cluster}</span>
                ${isSelected ? '<span class="checkmark">✓</span>' : ''}
            </div>
        `;
    }).join('');

    // Add click handlers
    listEl.querySelectorAll('.cluster-option').forEach(option => {
        option.addEventListener('click', (e) => {
            e.stopPropagation();
            const clusterName = option.dataset.cluster;
            
            if (selectedClusters.has(clusterName)) {
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