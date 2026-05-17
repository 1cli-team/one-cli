import { useTokenStore } from "@/store/secure";
import { AppResponse } from "@/types";
import axios from "axios";

export const refreshTokenKey = "/auth/refresh_token";
export const refreshToken = async () => {
  const { token } = useTokenStore.getState();
  // 这里需要使用 axios 避免循环引用
  const res = await axios.post<AppResponse<string>>(
    refreshTokenKey,
    {},
    {
      headers: {
        token,
      },
    },
  );
  if (!res.data.data) {
    throw new Error("No token data");
  }
  return res.data.data;
};
