import { STAGE_LABELS, PROGRESS_STAGES } from './constants.js';
import { cleanAnsiEscapes } from './utils.js';

export class ProgressManager {
    constructor(progressOverview, progressBarFill, progressText, progressPercentage) {
        this.progressOverview = progressOverview;
        this.progressBarFill = progressBarFill;
        this.progressText = progressText;
        this.progressPercentage = progressPercentage;
        this.currentProgressItems = new Map();
        this.stageStatus = {
            prerequisites: 'pending',
            settings: 'pending',
            build: 'pending',
            deploy: 'pending',
            patch: 'pending'
        };
    }

    initialize() {
        this.progressOverview.innerHTML = '';
        this.currentProgressItems.clear();

        this.stageStatus = {
            prerequisites: 'pending',
            settings: 'pending',
            build: 'pending',
            deploy: 'pending',
            patch: 'pending'
        };

        PROGRESS_STAGES.forEach(stage => {
            this.addProgressItem(stage.id, stage.label, stage.status);
        });
    }

    addProgressItem(id, label, status, details = '') {
        let item = this.currentProgressItems.get(id);

        if (!item) {
            item = document.createElement('div');
            item.className = 'progress-item';
            item.innerHTML = `
                <div class="progress-status status-${status}"></div>
                <div class="progress-label">${label}</div>
                <div class="progress-details">${details}</div>
            `;
            this.progressOverview.appendChild(item);
            this.currentProgressItems.set(id, item);
        } else {
            this.updateProgressItem(id, label, status, details);
        }
    }

    updateProgressItem(id, label, status, details = '') {
        const item = this.currentProgressItems.get(id);
        if (!item) return;

        const statusDiv = item.querySelector('.progress-status');
        const labelDiv = item.querySelector('.progress-label');
        const detailsDiv = item.querySelector('.progress-details');

        statusDiv.className = `progress-status status-${status}`;
        labelDiv.textContent = label;
        detailsDiv.textContent = details;
    }

    addServiceProgressItem(service, stage, status, details = '') {
        const cleanService = cleanAnsiEscapes(service);
        const id = `${cleanService}-${stage}`;
        const label = `${cleanService} - ${stage.charAt(0).toUpperCase() + stage.slice(1)}`;
        this.addProgressItem(id, label, status, details);
    }

    updateStageProgress(stage, status) {
        if (this.stageStatus[stage] !== status) {
            this.stageStatus[stage] = status;

            if (STAGE_LABELS[stage]) {
                const existingItem = this.currentProgressItems.get(stage);
                const existingDetails = existingItem ?
                    existingItem.querySelector('.progress-details').textContent : '';

                let detailsToShow = '';
                if (stage === 'prerequisites' || stage === 'settings') {
                    detailsToShow = existingDetails;
                } else if (status === 'running' || status === 'error') {
                    detailsToShow = existingDetails;
                }

                this.updateProgressItem(stage, STAGE_LABELS[stage], status, detailsToShow);
            }
        }
    }

    updateProgressBar(percentage, text) {
        this.progressBarFill.style.width = percentage + '%';
        this.progressText.textContent = text;
        this.progressPercentage.textContent = percentage + '%';
    }

    handleProgressUpdate(data) {
        console.log('Progress update received:', data);

        if (data.stage && data.status) {
            if (data.service) {
                const cleanService = cleanAnsiEscapes(data.service);
                this.addServiceProgressItem(cleanService, data.stage, data.status, data.details);

                if (data.status === 'running') {
                    this.updateStageProgress(data.stage, 'running');
                    this.updateProgressBarForStage(data.stage, 'running');
                } else if (data.status === 'success') {
                    this.updateStageProgress(data.stage, 'success');
                } else if (data.status === 'error') {
                    this.updateStageProgress(data.stage, 'error');
                }
            } else {
                this.updateProgressItem(data.stage, data.message, data.status, data.details);
                this.updateStageProgress(data.stage, data.status);
                this.updateProgressBarForStage(data.stage, data.status);
            }
        }
    }

    updateProgressBarForStage(stage, status) {
        const progressMap = {
            prerequisites: { running: 2, success: 5 },
            settings: { running: 7, success: 10 },
            build: { running: 30, success: 50 },
            deploy: { running: 60, success: 80 },
            patch: { running: 85, success: 100 }
        };

        const textMap = {
            prerequisites: { running: 'Checking prerequisites...', success: 'Prerequisites completed' },
            settings: { running: 'Updating Maven settings...', success: 'Maven settings updated' },
            build: { running: 'Building microservices...', success: 'Build completed' },
            deploy: { running: 'Creating Docker images...', success: 'Docker images created' },
            patch: { running: 'Deploying to Kubernetes...', success: 'Deployment completed' }
        };

        if (progressMap[stage] && progressMap[stage][status]) {
            this.updateProgressBar(
                progressMap[stage][status],
                textMap[stage][status] || `${stage} ${status}`
            );
        }

        if (status === 'error') {
            this.updateProgressBar(0, `${stage} failed`);
        }
    }
}
