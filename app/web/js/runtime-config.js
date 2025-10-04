const ENDPOINT_KEYS = [
    'storageJenkinsBaseUrl',
    'storageJenkinsJobPath',
    'customizationJenkinsBaseUrl',
    'scalingJenkinsJobPath',
    'bitbucketBaseUrl',
    'bitbucketProjectKey',
    'bitbucketCustomizationRepo',
    'nexusSearchUrl',
    'nexusRepositoryBaseUrl',
    'nexusInternalProxyBaseUrl',
    'hfParentRepoUrl'
];

export const RUNTIME_CONFIG = {};

export async function loadRuntimeConfig() {
    try {
        const response = await fetch('/api/config/public');
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }
        const data = await response.json();
        const endpoints = extractEndpoints(data);
        for (const key of ENDPOINT_KEYS) {
            RUNTIME_CONFIG[key] = endpoints[key];
        }
    } catch (error) {
        console.error('Failed to load runtime configuration', error);
        throw error;
    }
}

function extractEndpoints(payload) {
    if (!payload || typeof payload !== 'object' || !payload.endpoints) {
        throw new Error('Runtime configuration payload missing "endpoints" section');
    }
    const endpoints = payload.endpoints;
    const result = {};

    for (const key of ENDPOINT_KEYS) {
        const rawValue = endpoints[key];
        if (typeof rawValue !== 'string') {
            throw new Error(`Runtime configuration key "${key}" missing or not a string`);
        }
        const trimmed = rawValue.trim();
        if (trimmed === '') {
            throw new Error(`Runtime configuration key "${key}" is empty`);
        }
        result[key] = trimmed;
    }

    return result;
}

export function storageJobRoot() {
    return joinUrl(requireConfigValue('storageJenkinsBaseUrl'), requireConfigValue('storageJenkinsJobPath'));
}

export function storageJobUrl(...parts) {
    const root = storageJobRoot();
    if (parts.length === 0) {
        return ensureTrailingSlash(root);
    }
    let result = root;
    for (const part of parts) {
        if (!part) continue;
        result = joinUrl(result, part);
    }
    return result;
}

function joinUrl(base, path) {
    if (!base) return path || '';
    if (!path) return base;
    return `${trimTrailingSlash(base)}/${trimLeadingSlash(path)}`;
}

function ensureTrailingSlash(value) {
    if (!value.endsWith('/')) {
        return `${value}/`;
    }
    return value;
}

export function getRuntimeConfigValue(key) {
    if (!ENDPOINT_KEYS.includes(key)) {
        throw new Error(`Runtime configuration key "${key}" is not recognised`);
    }
    return requireConfigValue(key);
}

function requireConfigValue(key) {
    const value = RUNTIME_CONFIG[key];
    if (typeof value !== 'string' || value === '') {
        throw new Error(`Runtime configuration key "${key}" is not initialised`);
    }
    return value;
}

function trimLeadingSlash(value) {
    return value.replace(/^\/+/, '');
}

function trimTrailingSlash(value) {
    return value.replace(/\/+$/, '');
}
