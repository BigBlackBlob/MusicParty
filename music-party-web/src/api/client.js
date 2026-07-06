import axios from 'axios';
import { getConnectionProfile, onConnectionProfileChange } from './connectionProfile';

const client = axios.create({
    baseURL: getConnectionProfile().baseUrl,
    timeout: 10000
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
