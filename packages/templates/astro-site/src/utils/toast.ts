export type ToastType = "success" | "warning" | "error" | "info";

export interface Toast {
  id: string;
  type: ToastType;
  title: string;
  message?: string;
  duration?: number;
  persistent?: boolean;
}

export interface ToastOptions {
  title: string;
  message?: string;
  duration?: number;
  persistent?: boolean;
}

class ToastManager {
  private toasts: Toast[] = [];
  private container: HTMLElement | null = null;
  private listeners: Array<(toasts: Toast[]) => void> = [];

  constructor() {
    if (typeof window !== "undefined") {
      this.initContainer();
    }
  }

  private initContainer() {
    // 检查是否已存在容器
    this.container = document.getElementById("toast-container");
    if (!this.container) {
      this.container = document.createElement("div");
      this.container.id = "toast-container";
      this.container.className = `
				fixed top-4 right-4 z-50 flex flex-col gap-3 
				max-w-sm w-full pointer-events-none
			`.trim();
      document.body.appendChild(this.container);
    }
  }

  private generateId(): string {
    return `toast-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
  }

  private addToast(type: ToastType, options: ToastOptions): string {
    const id = this.generateId();
    const toast: Toast = {
      id,
      type,
      ...options,
      duration: options.duration ?? (type === "error" ? 0 : 5000), // 错误消息默认不自动消失
    };

    this.toasts.push(toast);
    this.render();
    this.notifyListeners();

    // 自动移除（如果不是持久化的）
    if (!toast.persistent && toast.duration && toast.duration > 0) {
      setTimeout(() => {
        this.remove(id);
      }, toast.duration);
    }

    return id;
  }

  private render() {
    if (!this.container) return;

    this.container.innerHTML = this.toasts.map((toast) => this.createToastHTML(toast)).join("");

    // 重新绑定关闭按钮事件
    this.toasts.forEach((toast) => {
      const closeBtn = document.getElementById(`close-${toast.id}`);
      if (closeBtn) {
        closeBtn.onclick = () => this.remove(toast.id);
      }
    });
  }

  private createToastHTML(toast: Toast): string {
    const { type, id, title, message } = toast;

    const typeStyles = {
      success: {
        container: "bg-success-50 border-success-200 text-success-800",
        icon: "✅",
        iconBg: "bg-success-100",
      },
      warning: {
        container: "bg-warning-50 border-warning-200 text-warning-800",
        icon: "⚠️",
        iconBg: "bg-warning-100",
      },
      error: {
        container: "bg-error-50 border-error-200 text-error-800",
        icon: "❌",
        iconBg: "bg-error-100",
      },
      info: {
        container: "bg-primary-50 border-primary-200 text-primary-800",
        icon: "ℹ️",
        iconBg: "bg-primary-100",
      },
    };

    const style = typeStyles[type];

    return `
			<div 
				class="
					${style.container} 
					border rounded-lg p-4 shadow-lg
					pointer-events-auto transform transition-all duration-300
					animate-slide-up max-w-full
				"
				id="toast-${id}"
			>
				<div class="flex items-start gap-3">
					<div class="flex-shrink-0 w-6 h-6 ${style.iconBg} rounded-full flex items-center justify-center text-sm">
						${style.icon}
					</div>
					<div class="flex-1 min-w-0">
						<div class="font-semibold text-sm">${title}</div>
						${message ? `<div class="text-sm mt-1 opacity-90">${message}</div>` : ""}
					</div>
					<button 
						id="close-${id}"
						class="flex-shrink-0 ml-2 text-lg leading-none opacity-60 hover:opacity-100 transition-opacity"
						aria-label="关闭通知"
					>
						×
					</button>
				</div>
			</div>
		`;
  }

  private notifyListeners() {
    this.listeners.forEach((listener) => {
      listener([...this.toasts]);
    });
  }

  public success(options: ToastOptions): string {
    return this.addToast("success", options);
  }

  public warning(options: ToastOptions): string {
    return this.addToast("warning", options);
  }

  public error(options: ToastOptions): string {
    return this.addToast("error", {
      ...options,
      persistent: options.persistent ?? true, // 错误消息默认持久化
    });
  }

  public info(options: ToastOptions): string {
    return this.addToast("info", options);
  }

  public remove(id: string): boolean {
    const index = this.toasts.findIndex((toast) => toast.id === id);
    if (index > -1) {
      // 添加退出动画
      const toastElement = document.getElementById(`toast-${id}`);
      if (toastElement) {
        toastElement.style.transform = "translateX(100%)";
        toastElement.style.opacity = "0";
        setTimeout(() => {
          this.toasts.splice(index, 1);
          this.render();
          this.notifyListeners();
        }, 300);
      } else {
        this.toasts.splice(index, 1);
        this.render();
        this.notifyListeners();
      }
      return true;
    }
    return false;
  }

  public clear(): void {
    this.toasts = [];
    this.render();
    this.notifyListeners();
  }

  public getToasts(): Toast[] {
    return [...this.toasts];
  }

  public subscribe(listener: (toasts: Toast[]) => void): () => void {
    this.listeners.push(listener);
    return () => {
      const index = this.listeners.indexOf(listener);
      if (index > -1) {
        this.listeners.splice(index, 1);
      }
    };
  }
}

// 创建全局实例
export const toast = new ToastManager();

// 便捷方法
export const showToast = {
  success: (title: string, message?: string) => toast.success({ title, message }),
  warning: (title: string, message?: string) => toast.warning({ title, message }),
  error: (title: string, message?: string) => toast.error({ title, message }),
  info: (title: string, message?: string) => toast.info({ title, message }),
};

// 错误处理辅助函数
export const handleError = (error: unknown, context?: string) => {
  const errorMessage = error instanceof Error ? error.message : String(error);
  const title = context ? `${context} 失败` : "发生错误";

  console.error(title, error);
  toast.error({
    title,
    message: errorMessage,
    persistent: true,
  });
};
