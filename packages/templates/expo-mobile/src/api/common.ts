import { axiosAuth, axiosPublic } from "@/lib/axios";
import { AppResponse } from "@/types";
import { CommonParams } from "@/types/common";

export const commonAuthApiKey = "/common";
export const commonAuthApi = async (data: CommonParams) => {
  const resp = await axiosAuth.post<AppResponse<string>>(commonAuthApiKey, data);
  return resp.data.data;
};

export const commonPublicApiKey = "/common/public";
export const commonPublicApi = async () => {
  const resp = await axiosPublic.post<AppResponse<string>>(commonPublicApiKey);
  return resp.data.data;
};
