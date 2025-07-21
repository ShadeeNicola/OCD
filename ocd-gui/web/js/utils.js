export function ansiToHtml(text) {
    const ansiColors = {
        // Standard colors
        '30': 'color: #000000',  // black
        '31': 'color: #e74c3c',  // red
        '32': 'color: #27ae60',  // green  
        '33': 'color: #f39c12',  // yellow
        '34': 'color: #3498db',  // blue
        '35': 'color: #9b59b6',  // magenta
        '36': 'color: #1abc9c',  // cyan
        '37': 'color: #ecf0f1',  // white/gray
        
        // Bright colors
        '90': 'color: #7f8c8d',  // bright black (gray)
        '91': 'color: #e74c3c',  // bright red
        '92': 'color: #2ecc71',  // bright green
        '93': 'color: #f1c40f',  // bright yellow
        '94': 'color: #74c0fc',  // bright blue
        '95': 'color: #e91e63',  // bright magenta
        '96': 'color: #00bcd4',  // bright cyan
        '97': 'color: #ffffff'   // bright white
    };

    // Handle both \033[ and \x1b[ escape sequences
    return text.replace(/(\033\[|\x1b\[)([0-9;]*)m/g, (match, prefix, codes) => {
        if (codes === '0' || codes === '') return '</span>';
        
        // Handle multiple codes separated by semicolons
        const codeArray = codes.split(';');
        let styles = [];
        
        for (let code of codeArray) {
            if (ansiColors[code]) {
                styles.push(ansiColors[code]);
            }
        }
        
        if (styles.length > 0) {
            return `<span style="${styles.join('; ')}">`;
        }
        
        return '';
    });
}

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