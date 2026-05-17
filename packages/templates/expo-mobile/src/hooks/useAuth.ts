import { configSelector, useConfigStore } from "@/store/config";
import { tokenSelector, useTokenStore } from "@/store/secure";
import { useShallow } from "zustand/react/shallow";
import useSWRMutation from "swr/mutation";
import { login, loginKey } from "@/api/auth";
import { LoginParams } from "@/types/auth";
import { useMemoizedFn } from "ahooks";

export function useAuth() {
  const { isLogin, setIsLogin } = useConfigStore(useShallow(configSelector));
  const { setToken } = useTokenStore(useShallow(tokenSelector));

  // login
  const { trigger } = useSWRMutation(loginKey, (key, { arg }: { arg: LoginParams }) => {
    return login(arg);
  });

  const userLogin = useMemoizedFn(async (code: string) => {
    try {
      const token = await trigger({ code });
      setIsLogin(true);
      setToken(token);
    } catch (error) {
      console.error(error);
    }
  });

  const userLogout = useMemoizedFn(() => {
    setIsLogin(false);
    setToken(null);
  });

  return {
    isLogin,
    userLogin,
    userLogout,
  };
}
