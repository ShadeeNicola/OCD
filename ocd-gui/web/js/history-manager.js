import { CONFIG } from './constants.js';
import { normalizePath } from './utils.js';

export class HistoryManager {
    constructor(historyDropdown, folderInput) {
        this.historyDropdown = historyDropdown;
        this.folderInput = folderInput;
    }

    getHistory() {
        try {
            const history = localStorage.getItem(CONFIG.HISTORY_KEY);
            return history ? JSON.parse(history) : [];
        } catch (e) {
            console.warn('Could not load history from localStorage:', e);
            return [];
        }
    }

    saveHistory(history) {
        try {
            localStorage.setItem(CONFIG.HISTORY_KEY, JSON.stringify(history));
        } catch (e) {
            console.warn('Could not save history to localStorage:', e);
        }
    }

    addToHistory(path) {
        if (!path || !path.trim()) return;

        const normalizedPath = normalizePath(path.trim());
        let history = this.getHistory();

        history = history.filter(item => normalizePath(item) !== normalizedPath);
        history.unshift(path.trim());

        if (history.length > CONFIG.MAX_HISTORY_ITEMS) {
            history = history.slice(0, CONFIG.MAX_HISTORY_ITEMS);
        }

        this.saveHistory(history);
    }

    removeFromHistory(path) {
        const normalizedPath = normalizePath(path);
        let history = this.getHistory();
        history = history.filter(item => normalizePath(item) !== normalizedPath);
        this.saveHistory(history);
        this.showHistoryDropdown();
    }

    showHistoryDropdown() {
        const history = this.getHistory();
        this.historyDropdown.innerHTML = '';

        if (history.length === 0) {
            this.showEmptyHistory();
        } else {
            this.showHistoryItems(history);
        }

        this.historyDropdown.style.display = 'block';
    }

    showEmptyHistory() {
        const emptyDiv = document.createElement('div');
        emptyDiv.className = 'history-empty';
        emptyDiv.textContent = 'No recent folders';
        this.historyDropdown.appendChild(emptyDiv);
    }

    showHistoryItems(history) {
        history.forEach(path => {
            const itemDiv = this.createHistoryItem(path);
            this.historyDropdown.appendChild(itemDiv);
        });
    }

    createHistoryItem(path) {
        const itemDiv = document.createElement('div');
        itemDiv.className = 'history-item';

        const pathSpan = document.createElement('span');
        pathSpan.className = 'history-path';
        pathSpan.textContent = path;
        pathSpan.title = path;

        const deleteBtn = document.createElement('button');
        deleteBtn.className = 'history-delete';
        deleteBtn.textContent = '×';
        deleteBtn.title = 'Remove from history';

        pathSpan.addEventListener('click', (e) => {
            e.stopPropagation();
            this.folderInput.value = path;
            this.folderInput.dispatchEvent(new Event('input'));
            this.hideHistoryDropdown();
        });

        deleteBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            this.removeFromHistory(path);
        });

        itemDiv.appendChild(pathSpan);
        itemDiv.appendChild(deleteBtn);
        return itemDiv;
    }

    hideHistoryDropdown() {
        this.historyDropdown.style.display = 'none';
    }
}