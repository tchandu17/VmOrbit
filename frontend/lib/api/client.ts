import axios from "axios";

const api = axios.create({
  baseURL: "/api/proxy/api",
  headers: { "Content-Type": "application/json" },
});

// Separate instance for token refresh — avoids circular interceptor loops
const refreshApi = axios.create({
  baseURL: "/api/proxy/api",
  headers: { "Content-Type": "application/json" },
});

// Track if a refresh is already in progress to avoid parallel refresh calls
let isRefreshing = false;
let refreshSubscribers: Array<(token: string) => void> = [];

function subscribeTokenRefresh(cb: (token: string) => void) {
  refreshSubscribers.push(cb);
}

function onRefreshed(token: string) {
  refreshSubscribers.forEach(cb => cb(token));
  refreshSubscribers = [];
}

// Attach JWT on every request
api.interceptors.request.use((config) => {
  if (typeof window !== "undefined") {
    const token = localStorage.getItem("access_token");
    if (token) config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Handle 401 with automatic token refresh, and extract backend error messages
api.interceptors.response.use(
  (r) => r,
  async (err) => {
    const originalRequest = err.config;

    if (err.response?.status === 401 && typeof window !== "undefined" && !originalRequest._retry) {
      const refreshToken = localStorage.getItem("refresh_token");

      // No refresh token — clear and redirect to login
      if (!refreshToken) {
        localStorage.removeItem("access_token");
        localStorage.removeItem("refresh_token");
        localStorage.removeItem("vmorbit_user");
        window.location.href = "/login";
        return Promise.reject(err);
      }

      // If already refreshing, queue this request until refresh completes
      if (isRefreshing) {
        return new Promise((resolve) => {
          subscribeTokenRefresh((newToken: string) => {
            originalRequest.headers.Authorization = `Bearer ${newToken}`;
            resolve(api(originalRequest));
          });
        });
      }

      originalRequest._retry = true;
      isRefreshing = true;

      try {
        const res = await refreshApi.post<{ success: boolean; data: { access_token: string; refresh_token: string } }>(
          "/v1/auth/refresh",
          { refresh_token: refreshToken }
        );
        const newAccessToken = res.data.data.access_token;
        const newRefreshToken = res.data.data.refresh_token;
        localStorage.setItem("access_token", newAccessToken);
        localStorage.setItem("refresh_token", newRefreshToken);
        api.defaults.headers.common["Authorization"] = `Bearer ${newAccessToken}`;
        onRefreshed(newAccessToken);
        originalRequest.headers.Authorization = `Bearer ${newAccessToken}`;
        return api(originalRequest);
      } catch {
        // Refresh failed — clear everything and redirect
        localStorage.removeItem("access_token");
        localStorage.removeItem("refresh_token");
        localStorage.removeItem("vmorbit_user");
        window.location.href = "/login";
        return Promise.reject(err);
      } finally {
        isRefreshing = false;
      }
    }

    // Replace the generic Axios message with the backend error field when available
    const backendError = err.response?.data?.error;
    if (backendError) {
      err.message = backendError;
    }
    return Promise.reject(err);
  }
);

export default api;
