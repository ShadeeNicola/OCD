export const CONFIG = {
    HISTORY_KEY: 'ocd-folder-history',
    MAX_HISTORY_ITEMS: 10,
    WEBSOCKET_TIMEOUT: 30000,
    STATUS_DISPLAY_TIME: 5000
};

export const STAGE_LABELS = {
    prerequisites: 'Connection Checks & Prerequisites',
    settings: 'Maven Settings XML Update',
    build: 'Building Microservices',
    deploy: 'Docker Image Creation',
    patch: 'Kubernetes Deployment'
};

export const PROGRESS_STAGES = [
    { id: 'prerequisites', label: 'Connection Checks & Prerequisites', status: 'pending' },
    { id: 'settings', label: 'Maven Settings XML Update', status: 'pending' },
    { id: 'build', label: 'Building Microservices', status: 'pending' },
    { id: 'deploy', label: 'Docker Image Creation', status: 'pending' },
    { id: 'patch', label: 'Kubernetes Deployment', status: 'pending' }
];