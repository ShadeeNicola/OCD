export function cleanAnsiEscapes(text) {
    return text.replace(/\x1b\[[0-9;]*m/g, '');
}

export function normalizePath(path) {
    return path.replace(/\\/g, '/');
}

export function createWebSocketUrl() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${protocol}//${window.location.host}/ws/deploy`;
}

export function generateTimestamp() {
    return new Date().toISOString().replace(/[:.]/g, '-');
}

export function downloadTextFile(content, filename) {
    const blob = new Blob([content], { type: 'text/plain' });
    const url = window.URL.createObjectURL(blob);

    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    window.URL.revokeObjectURL(url);
}