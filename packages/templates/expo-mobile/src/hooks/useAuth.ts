import { configSelector, useConfigStore } from "@/store/config";
import { tokenSelector, useTokenStore } from "@/store/secure";
import { useShallow } from "zustand/react/shallow";
import useSWRMutation from "swr/mutation";
import { login, loginKey } from "@/api/auth";
import { LoginParams } from "@/types/auth";
import { useCallback } from "react";

export function useAuth() {
  const { isLogin, setIsLogin } = useConfigStore(useShallow(configSelector));
  const { setToken } = useTokenStore(useShallow(tokenSelector));

  // login
  const { trigger } = useSWRMutation(loginKey, (key, { arg }: { arg: LoginParams }) => {
    return login(arg);
  });

  const userLogin = useCallback(
    async (code: string) => {
      try {
        const token = await trigger({ code });
        setIsLogin(true);
        setToken(token);
      } catch (error) {
        console.error(error);
      }
    },
    [trigger, setIsLogin, setToken],
  );

  const userLogout = useCallback(() => {
    setIsLogin(false);
    setToken(null);
  }, [setIsLogin, setToken]);

  return {
    isLogin,
    userLogin,
    userLogout,
  };
}
