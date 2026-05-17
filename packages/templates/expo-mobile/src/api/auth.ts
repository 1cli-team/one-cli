import { AppResponse } from "@/types";
import { LoginParams } from "@/types/auth";
import { axiosPublic } from "@/lib/axios";

export const loginKey = "/auth/login";
export const login = async (data: LoginParams) => {
  const res = await axiosPublic.post<AppResponse<string>>(loginKey, data);
  return res.data.data;
};
