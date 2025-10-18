import { areCredentialsConfigured, showSetupModal, getSavedCredentials } from './settings.js';

let buildsCache = [];
let currentComparison = null;
let trendsChartInstance = null;
let initialized = false;

export function initializeAutomationReports() {
    if (initialized) {
        return;
    }

    bindEventListeners();
    loadBuilds();
    initialized = true;
}

function bindEventListeners() {
    const refreshButton = document.getElementById('refresh-builds-btn');
    const compareButton = document.getElementById('compare-builds-btn');

    if (refreshButton) {
        refreshButton.addEventListener('click', loadBuilds);
    }

    if (compareButton) {
        compareButton.addEventListener('click', handleCompare);
    }
}

async function loadBuilds() {
    if (!ensureCredentials()) {
        return;
    }

    setAutomationStatus('info', 'Loading build history...', true);

    try {
        const credentials = getSavedCredentials();
        const response = await fetch('/api/automation/builds', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                limit: 20,
                username: credentials.username,
                token: credentials.token
            })
        });

        const data = await response.json();
        if (!response.ok || !data.success) {
            throw new Error(data.message || 'Failed to fetch builds');
        }

        buildsCache = data.data.builds || [];
        populateBuildDropdowns();

        if (buildsCache.length >= 2) {
            setAutomationStatus('success', 'Select builds to compare.', false);
        } else {
            setAutomationStatus('info', 'Not enough builds to compare yet.', false);
        }
    } catch (error) {
        console.error('Failed to load builds', error);
        setAutomationStatus('error', `Failed to load builds: ${error.message}`);
    }
}

function populateBuildDropdowns() {
    const selectA = document.getElementById('build-select-a');
    const selectB = document.getElementById('build-select-b');

    if (!selectA || !selectB) {
        return;
    }

    const options = buildsCache
        .slice()
        .sort((a, b) => b.number - a.number)
        .map(build => ({
            value: build.number,
            label: `#${build.number} – ${build.result || 'UNKNOWN'}`
        }));

    selectA.innerHTML = options.map(option => `<option value="${option.value}">${option.label}</option>`).join('');
    selectB.innerHTML = options.map(option => `<option value="${option.value}">${option.label}</option>`).join('');

    if (options.length >= 2) {
        selectA.selectedIndex = 1;
        selectB.selectedIndex = 0;
    }
}

async function handleCompare() {
    if (!ensureCredentials()) {
        return;
    }

    const selectA = document.getElementById('build-select-a');
    const selectB = document.getElementById('build-select-b');

    if (!selectA || !selectB) {
        return;
    }

    const buildA = parseInt(selectA.value, 10);
    const buildB = parseInt(selectB.value, 10);

    if (!buildA || !buildB || buildA === buildB) {
        setAutomationStatus('error', 'Please choose two different builds to compare.');
        return;
    }

    setAutomationStatus('info', 'Comparing builds...', true);
    toggleCompareControls(true);

    try {
        currentComparison = await fetchComparison(buildA, buildB);
        renderComparison(currentComparison);
        await loadTrends();
        setAutomationStatus('success', 'Comparison completed.');
    } catch (error) {
        console.error('Comparison failed', error);
        setAutomationStatus('error', `Comparison failed: ${error.message}`);
    } finally {
        toggleCompareControls(false);
    }
}

async function fetchComparison(buildA, buildB) {
    const credentials = getSavedCredentials();
    const response = await fetch('/api/automation/compare', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            buildA,
            buildB,
            username: credentials.username,
            token: credentials.token
        })
    });

    const data = await response.json();
    if (!response.ok || !data.success) {
        throw new Error(data.message || 'Comparison failed');
    }

    return data.data;
}

function renderComparison(comparison) {
    renderSummary(comparison.summary);
    renderRegressionTable(comparison.regressions);
    renderProgressionTable(comparison.progressions);
    renderChangeCandidates(comparison.changeCandidates);
}

function renderSummary(summary) {
    const summaryContainer = document.getElementById('automation-summary');
    if (!summaryContainer) {
        return;
    }

    summaryContainer.style.display = 'grid';
    setValue('summary-total-tests', summary.totalTestsB);
    setDelta('summary-total-tests-delta', summary.totalTestsB - summary.totalTestsA);

    setValue('summary-pass-rate', `${summary.passRateB.toFixed(1)}%`);
    const passDelta = summary.passRateB - summary.passRateA;
    setDelta('summary-pass-rate-delta', passDelta, 'percentage');

    setValue('summary-avg-duration', `${summary.avgDurationB.toFixed(2)}s`);
    const durationDelta = summary.avgDurationB - summary.avgDurationA;
    setDelta('summary-avg-duration-delta', durationDelta, 'seconds');

    setValue('summary-regressions', summary.regressionsCount);
    setValue('summary-progressions', `${summary.progressionsCount} Progressions`);
}

function renderRegressionTable(regressions) {
    const tableBody = document.getElementById('regressions-body');
    const card = document.getElementById('automation-regressions');
    if (!tableBody || !card) {
        return;
    }

    if (!regressions || regressions.length === 0) {
        card.style.display = 'none';
        tableBody.innerHTML = `<tr><td colspan="5" class="empty">No regressions detected</td></tr>`;
        return;
    }

    card.style.display = 'block';
    tableBody.innerHTML = regressions
        .map(test => `
            <tr>
                <td>
                    <span class="test-name">${escapeHTML(test.testName)}</span>
                    <span class="class-name">${escapeHTML(test.className)}</span>
                </td>
                <td>${statusTag(test.statusA)}</td>
                <td>${statusTag(test.statusB)}</td>
                <td>${durationDelta(test.durationA, test.durationB)}</td>
                <td>${escapeHTML(test.error || '')}</td>
            </tr>
        `)
        .join('');
}

function renderProgressionTable(progressions) {
    const tableBody = document.getElementById('progressions-body');
    const card = document.getElementById('automation-progressions');
    if (!tableBody || !card) {
        return;
    }

    if (!progressions || progressions.length === 0) {
        card.style.display = 'none';
        tableBody.innerHTML = `<tr><td colspan="5" class="empty">No progressions detected</td></tr>`;
        return;
    }

    card.style.display = 'block';
    tableBody.innerHTML = progressions
        .map(test => `
            <tr>
                <td>
                    <span class="test-name">${escapeHTML(test.testName)}</span>
                    <span class="class-name">${escapeHTML(test.className)}</span>
                </td>
                <td>${statusTag(test.statusA)}</td>
                <td>${statusTag(test.statusB)}</td>
                <td>${test.failedSince ? `#${test.failedSince}` : '—'}</td>
                <td>${durationDelta(test.durationA, test.durationB)}</td>
            </tr>
        `)
        .join('');
}

function renderChangeCandidates(groups) {
    const list = document.getElementById('change-candidates-list');
    const card = document.getElementById('automation-change-candidates');
    if (!list || !card) {
        return;
    }

    if (!groups || groups.length === 0) {
        card.style.display = 'none';
        list.innerHTML = '';
        return;
    }

    card.style.display = 'block';
    list.innerHTML = groups
        .map(group => `
            <div class="change-candidate">
                <h4>${escapeHTML(group.author)} (${group.commitCount} commits)</h4>
                <ul class="commit-list">
                    ${group.commits.map(commit => `
                        <li>
                            <span class="commit-id">${escapeHTML(commit.shortId)}</span>
                            <span>${escapeHTML(commit.message || '[no message]')}</span>
                        </li>
                    `).join('')}
                </ul>
            </div>
        `)
        .join('');
}

async function loadTrends() {
    if (!ensureCredentials()) {
        return;
    }

    const card = document.getElementById('automation-trends');
    if (!card) {
        return;
    }

    try {
        const credentials = getSavedCredentials();
        const response = await fetch('/api/automation/trends', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                numBuilds: 10,
                username: credentials.username,
                token: credentials.token
            })
        });

        const data = await response.json();
        if (!response.ok || !data.success || !data.data || !data.data.builds || data.data.builds.length === 0) {
            card.style.display = 'none';
            return;
        }

        card.style.display = 'block';
        renderTrendsChart(data.data.builds);
    } catch (error) {
        console.error('Failed to load trends', error);
        card.style.display = 'none';
    }
}

function renderTrendsChart(builds) {
    const canvas = document.getElementById('automation-trends-chart');
    if (!canvas) {
        return;
    }

    const ctx = canvas.getContext('2d');
    if (!ctx) {
        return;
    }

    if (trendsChartInstance) {
        trendsChartInstance.destroy();
    }

    const ordered = builds.slice().reverse();
    const labels = ordered.map(build => `#${build.buildNumber}`);
    const passData = ordered.map(build => build.passCount);
    const failData = ordered.map(build => build.failCount);
    const passRateData = ordered.map(build => build.passRate);

    trendsChartInstance = new Chart(ctx, {
        type: 'line',
        data: {
            labels,
            datasets: [
                {
                    label: 'Pass Rate %',
                    data: passRateData,
                    borderColor: '#10b981',
                    backgroundColor: 'rgba(16, 185, 129, 0.1)',
                    yAxisID: 'y-rate',
                    tension: 0.4,
                },
                {
                    label: 'Passed Tests',
                    data: passData,
                    borderColor: '#3b82f6',
                    backgroundColor: 'rgba(59, 130, 246, 0.1)',
                    yAxisID: 'y-count',
                    tension: 0.4,
                },
                {
                    label: 'Failed Tests',
                    data: failData,
                    borderColor: '#ef4444',
                    backgroundColor: 'rgba(239, 68, 68, 0.1)',
                    yAxisID: 'y-count',
                    tension: 0.4,
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            interaction: {
                mode: 'index',
                intersect: false,
            },
            scales: {
                'y-rate': {
                    type: 'linear',
                    display: true,
                    position: 'right',
                    title: {
                        display: true,
                        text: 'Pass Rate %'
                    },
                    min: 0,
                    max: 100,
                },
                'y-count': {
                    type: 'linear',
                    display: true,
                    position: 'left',
                    title: {
                        display: true,
                        text: 'Test Count'
                    },
                    grid: {
                        drawOnChartArea: false,
                    },
                }
            }
        }
    });
}

function ensureCredentials() {
    if (!areCredentialsConfigured()) {
        showSetupModal();
        return false;
    }
    return true;
}

function setAutomationStatus(type, message, loading = false) {
    const statusElement = document.getElementById('automation-status');
    if (!statusElement) {
        return;
    }

    statusElement.className = `status-message ${type}`;
    statusElement.textContent = message;
    statusElement.style.display = message ? 'block' : 'none';

    toggleCompareControls(loading);
}

function toggleCompareControls(disabled) {
    const compareButton = document.getElementById('compare-builds-btn');
    const refreshButton = document.getElementById('refresh-builds-btn');

    if (compareButton) {
        compareButton.disabled = disabled;
    }
    if (refreshButton) {
        refreshButton.disabled = disabled;
    }
}

function setValue(id, value) {
    const element = document.getElementById(id);
    if (element) {
        element.textContent = value;
    }
}

function setDelta(id, delta, format = 'number') {
    const element = document.getElementById(id);
    if (!element) {
        return;
    }

    let text = '';
    let cssClass = 'neutral';

    if (delta > 0) {
        text = formatDelta(delta, format, '+');
        cssClass = 'positive';
    } else if (delta < 0) {
        text = formatDelta(Math.abs(delta), format, '-');
        cssClass = 'negative';
    } else {
        text = formatDelta(0, format, '');
    }

    element.textContent = text;
    element.className = `delta ${cssClass}`;
}

function formatDelta(value, format, sign) {
    switch (format) {
        case 'percentage':
            return `${sign}${Math.abs(value).toFixed(1)} pp`;
        case 'seconds':
            return `${sign}${Math.abs(value).toFixed(2)}s`;
        default:
            return `${sign}${value}`;
    }
}

function statusTag(status) {
    const normalized = (status || '').toUpperCase();
    if (!normalized) {
        return '—';
    }

    const className = normalized === 'PASSED' ? 'pass' : normalized === 'FAILED' ? 'fail' : 'skip';
    return `<span class="status-tag ${className}">${escapeHTML(normalized)}</span>`;
}

function durationDelta(durationA, durationB) {
    if (typeof durationA !== 'number' || typeof durationB !== 'number') {
        return '—';
    }

    const delta = durationB - durationA;
    if (Math.abs(delta) < 0.001) {
        return '<span class="duration-delta neutral">0.00s</span>';
    }

    const cssClass = delta > 0 ? 'positive' : 'negative';
    const sign = delta > 0 ? '+' : '-';
    return `<span class="duration-delta ${cssClass}">${sign}${Math.abs(delta).toFixed(2)}s</span>`;
}

function escapeHTML(value) {
    const div = document.createElement('div');
    div.textContent = value || '';
    return div.innerHTML;
}
