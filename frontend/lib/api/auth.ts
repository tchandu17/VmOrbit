import api from "./client";
import type { ApiResponse, AuthTokens, User } from "@/types";

export const authApi = {
  login: (email: string, password: string) =>
    api.post<ApiResponse<AuthTokens>>("/v1/auth/login", { email, password }).then((r) => r.data.data),

  refresh: (refresh_token: string) =>
    api.post<ApiResponse<AuthTokens>>("/v1/auth/refresh", { refresh_token }).then((r) => r.data.data),

  logout: (refresh_token: string) =>
    api.post("/v1/auth/logout", { refresh_token }),

  me: () =>
    api.get<ApiResponse<User>>("/v1/auth/me").then((r) => r.data.data),
};
