import { getSavedBitbucketCredentials } from './settings.js';
import { loadCustomizationBranches } from './utils.js';

export function initializeHFAdoption() {
	const dropZone = document.getElementById('hf-drop-zone');
	const fileInput = document.getElementById('hf-file-input');
	const browseBtn = document.getElementById('hf-browse-btn');
	const parseBtn = document.getElementById('hf-parse-btn');
	const applyBtn = document.getElementById('hf-apply-btn');
	const cancelBtn = document.getElementById('hf-cancel-btn');
	const previewTableBody = document.getElementById('hf-preview-body');
	const statusMsg = document.getElementById('hf-status-message');
	const branchSearch = document.getElementById('hf-branch-search');
	const branchDropdown = document.getElementById('hf-branch-dropdown');
	const branchList = document.getElementById('hf-branch-list');
	const refreshBranchesBtn = document.getElementById('hf-refresh-branches-btn');
	let selectedBranch = '';

	if (!dropZone || !fileInput) return;

    let selectedFile = null;
    let versions = {};
    let lastParsedSubject = '';
	let allBranches = [];

	// Drag & drop handlers
	dropZone.addEventListener('dragover', (e) => {
		e.preventDefault();
		dropZone.classList.add('drag-over');
	});
	dropZone.addEventListener('dragleave', () => dropZone.classList.remove('drag-over'));
	dropZone.addEventListener('drop', (e) => {
		e.preventDefault();
		dropZone.classList.remove('drag-over');
		if (e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files.length) {
			setFile(e.dataTransfer.files[0]);
		}
	});

	// Browse button
	browseBtn && browseBtn.addEventListener('click', () => fileInput.click());
	fileInput.addEventListener('change', () => {
		if (fileInput.files && fileInput.files[0]) setFile(fileInput.files[0]);
	});

    function setFile(file) {
        const lower = file.name.toLowerCase();
        if (!lower.endsWith('.eml') && !lower.endsWith('.msg')) {
            showStatus('Please select an .eml or .msg file', 'error');
            return;
        }
		selectedFile = file;
		showStatus(`Selected file: ${file.name}`, 'success');
	}

    parseBtn && parseBtn.addEventListener('click', async () => {
		if (!selectedFile) {
			showStatus('Please select an .eml file first', 'error');
			return;
		}
		try {
			parseBtn.disabled = true;
			parseBtn.textContent = 'Parsing...';
			const form = new FormData();
			form.append('file', selectedFile);
            const res = await fetch('/api/hf/parse-email', { method: 'POST', body: form });
			const data = await res.json();
			if (data.success) {
				versions = data.versions || {};
				renderPreview(versions);
				showStatus('Email parsed successfully', 'success');
				if (applyBtn) applyBtn.disabled = false;
                if (data.subject) lastParsedSubject = data.subject;
			} else {
				showStatus(data.message || 'Failed to parse email', 'error');
			}
		} catch (err) {
			showStatus(err.message || 'Error parsing email', 'error');
		} finally {
			parseBtn.disabled = false;
			parseBtn.textContent = 'Parse Email';
		}
	});

    applyBtn && applyBtn.addEventListener('click', async () => {
		if (!selectedBranch) { showStatus('Select a branch first', 'error'); return; }
		if (!versions || Object.keys(versions).length === 0) { showStatus('Parse an email first', 'error'); return; }
		try {
			applyBtn.disabled = true;
			applyBtn.textContent = 'Diffing...';
			const repoURL = 'https://ossbucket:7990/scm/attsvo/parent-pom.git';
			const creds = getSavedBitbucketCredentials();
            const payload = {
				repo_url: repoURL,
				branch: selectedBranch,
				versions: versions,
                dry_run: true,
                debug: true,
                commit_msg: '',
                // pass subject for commit title enrichment
                // best-effort: try to read from last parse response stored in DOM
			};
			if (creds && creds.username && creds.token) {
				payload.username = creds.username;
				payload.token = creds.token;
			}
            const headers = { 'Content-Type': 'application/json' };
            if (lastParsedSubject) headers['X-HF-Subject'] = lastParsedSubject;
            let res = await fetch('/api/hf/update-pom', {
				method: 'POST',
                headers: headers,
				body: JSON.stringify(payload)
			});
			let data = await res.json();
			if (!data.success) { showStatus(data.message || 'Dry-run failed', 'error'); return; }
            // Apply immediately (no confirmation step)
            applyBtn.disabled = true;
            applyBtn.textContent = 'Applying...';
            payload.dry_run = false;
            res = await fetch('/api/hf/update-pom', { method: 'POST', headers: headers, body: JSON.stringify(payload) });
            data = await res.json();
            if (data.success) {
                showStatus('POM updated and pushed successfully', 'success');
            } else {
                showStatus(data.message || 'Apply failed', 'error');
            }
		} catch (e) {
			showStatus(e.message || 'Error applying changes', 'error');
		} finally {
			applyBtn.disabled = false;
			applyBtn.textContent = 'Apply Changes';
		}
	});

	function renderPreview(map) {
		if (!previewTableBody) return;
		previewTableBody.innerHTML = '';
		const order = [
			['cmn_dop.version', 'CMN-DOP Chart'],
			['core.version', 'Core artifacts version'],
			['tmf622.version', 'tmf622 version'],
			['cmn.common.version', 'CMN Common Version'],
			['cmn.version', 'CMN Customization Version'],
			['cmn.snow.version', 'CMN Snow Version']
		];
		for (const [prop, label] of order) {
			const value = map[prop] || '';
			const tr = document.createElement('tr');
			tr.innerHTML = `<td>${label}</td><td><code>${prop}</code></td><td>${value}</td>`;
			previewTableBody.appendChild(tr);
		}
	}

	function showStatus(message, type) {
		if (!statusMsg) return;
		statusMsg.textContent = message;
		statusMsg.className = `status-message status-${type}`;
		statusMsg.style.display = 'block';
	}

	// Branch list loading (reuse customization endpoint)
	refreshBranchesBtn && refreshBranchesBtn.addEventListener('click', loadBranches);
	branchSearch && branchSearch.addEventListener('click', () => toggleBranchDropdown());

    async function loadBranches() {
        if (!branchSearch || !branchList) return;
        const creds = getSavedBitbucketCredentials();
        allBranches = await loadCustomizationBranches(creds, branchList, branchSearch);
        // Re-bind click handlers after repopulation
        Array.from(branchList.querySelectorAll('.branch-option')).forEach(el => {
            el.addEventListener('click', () => {
                const name = el.dataset.branch || el.textContent;
                branchSearch.value = '';
                branchSearch.placeholder = `Selected: ${name}`;
                selectedBranch = name;
                toggleBranchDropdown(false);
            });
        });
    }

	function populateBranchDropdown() {
		if (!branchList) return;
		branchList.innerHTML = '';
		allBranches.forEach(b => {
			const option = document.createElement('div');
			option.className = 'branch-option';
			option.textContent = b.name;
			option.addEventListener('click', () => {
				branchSearch.value = '';
				branchSearch.placeholder = `Selected: ${b.name}`;
				selectedBranch = b.name;
				toggleBranchDropdown(false);
			});
			branchList.appendChild(option);
		});
	}

	function toggleBranchDropdown(force) {
		if (!branchDropdown || !branchSearch) return;
		const wantOpen = force === undefined ? branchDropdown.style.display !== 'block' : force;
		branchDropdown.style.display = wantOpen ? 'block' : 'none';
	}

	// Initial load
	loadBranches();
}


