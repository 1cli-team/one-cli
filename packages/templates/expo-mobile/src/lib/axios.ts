import axios from "axios";
import { unstable_batchedUpdates } from "react-native";

import { AxiosRespError, TokenExpiredError } from "./error";

import { getAuthToken, hasJwtNotExpired } from "./auth/jwt";
import { useSessionStore } from "@/store/session";
import { BASE_API_URL } from "@/constants/api";

export const axiosPublic = axios.create({
  baseURL: `${BASE_API_URL}/api/v1`,
});

axiosPublic.interceptors.request.use(
  async (config) => {
    return config;
  },
  (error) => {
    return Promise.reject(error);
  },
);

axiosPublic.interceptors.response.use(
  (response) => {
    if (response?.data?.code !== 0) {
      const error = new AxiosRespError(response.data.msg || "Unknown error", response);
      return Promise.reject(error);
    }
    return response;
  },
  (error) => {
    return Promise.reject(error);
  },
);

export const axiosAuth = axios.create({
  baseURL: `${BASE_API_URL}/api/v1`,
});

axiosAuth.interceptors.request.use(
  async (config) => {
    try {
      const token = await getAuthToken();
      if (token && hasJwtNotExpired(token)) {
        config.headers.Authorization = `Bearer ${token}`;
      } else {
        unstable_batchedUpdates(() => {
          useSessionStore.getState().setLoginModalOpen(true);
        });
        if (token && !hasJwtNotExpired(token)) {
          return Promise.reject(new TokenExpiredError("Authentication has expired"));
        } else {
          return Promise.reject(new Error("Authentication is invalid"));
        }
      }
      return config;
    } catch (error) {
      return Promise.reject(error);
    }
  },
  (error) => {
    return Promise.reject(error);
  },
);

axiosAuth.interceptors.response.use(
  (response) => {
    if (response?.data?.code !== 0) {
      const error = new AxiosRespError(response.data.msg || "Unknown error", response);
      return Promise.reject(error);
    }
    return response;
  },
  (error) => {
    if (error.response?.status === 401) {
      // signOut().catch()
    }
    return Promise.reject(error);
  },
);
