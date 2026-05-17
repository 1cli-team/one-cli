import { Buffer } from "buffer";
import { unstable_batchedUpdates } from "react-native";

import { refreshToken } from "@/api/token";
import { useTokenStore } from "@/store/secure";

interface JwtPayload {
  exp: number;
  iat: number;
  aud: string;
  iss: string;
  sub: string;
  auth_time: number;
}

// 读取 token
export const getAuthToken = async (): Promise<string | null> => {
  try {
    const { token } = useTokenStore.getState();
    if (token && isJwtExpiringSoon(token)) {
      try {
        const token = await refreshToken();
        if (!token) {
          throw new Error("No refresh token data");
        }
        await setAuthToken(token);
      } catch (error) {
        console.error("Failed to refresh token:", error);
        await removeAuthToken();
      }
    }
    return token;
  } catch (error) {
    console.error("Error retrieving token:", error);
    return null;
  }
};

// 保存 token
export const setAuthToken = async (token: string): Promise<void> => {
  try {
    unstable_batchedUpdates(() => {
      useTokenStore.getState().setToken(token);
    });
  } catch (error) {
    console.error("Error saving token:", error);
  }
};

// 移除 token
export const removeAuthToken = async (): Promise<void> => {
  try {
    unstable_batchedUpdates(() => {
      useTokenStore.getState().setToken(null);
    });
  } catch (error) {
    console.error("Error removing token:", error);
  }
};

// 解析 JWT payload
export const parseJwtPayload = (token: string): JwtPayload => {
  try {
    const base64Url = token.split(".")[1];
    const base64 = base64Url.replace(/-/g, "+").replace(/_/g, "/");
    const jsonPayload = Buffer.from(base64, "base64").toString("utf8");
    return JSON.parse(jsonPayload);
  } catch (error) {
    console.error("Error parsing JWT payload:", error);
    throw new Error("Invalid token format");
  }
};

// 检查 JWT 是否未过期
export const hasJwtNotExpired = (token: string): boolean => {
  try {
    const payload = parseJwtPayload(token);
    if (payload.exp) {
      const currentTime = Math.floor(Date.now() / 1000);
      if (currentTime >= payload.exp) {
        return false;
      }
    }
    return true;
  } catch (error) {
    console.error("Error checking token expiration:", error);
    return false;
  }
};

// 检查 JWT 是否即将过期
const isJwtExpiringSoon = (token: string): boolean => {
  try {
    const payload = parseJwtPayload(token);
    if (payload.exp) {
      const currentTime = Math.floor(Date.now() / 1000);
      return payload.exp - currentTime > 0 && payload.exp - currentTime < 60 * 5;
    }
    return true; // 如果没有 exp 字段，假设即将过期
  } catch (error) {
    console.error("Error checking if token is expiring soon:", error);
    return false;
  }
};
