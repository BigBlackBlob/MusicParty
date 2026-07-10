import axios from 'axios';
import { getConnectionProfile, onConnectionProfileChange } from './connectionProfile';

const client = axios.create({
    baseURL: getConnectionProfile().baseUrl,
    timeout: 10000,
    withCredentials: true
});

client.interceptors.request?.use((config) => {
    const csrfToken = document.cookie.split('; ').find(value => value.startsWith('MP_CSRF='))?.split('=').slice(1).join('=');
    if (csrfToken && !['get', 'head', 'options'].includes((config.method || 'get').toLowerCase())) {
        config.headers = { ...config.headers, 'X-CSRF-Token': decodeURIComponent(csrfToken) };
    }
    return config;
});

onConnectionProfileChange((profile) => {
    client.defaults = client.defaults || {};
    client.defaults.baseURL = profile.baseUrl;
});

// 响应拦截器：可以在这里统一处理 401/403 等错误
client.interceptors.response.use(
    res => res.data,
    error => Promise.reject(error)
);

export default client;
