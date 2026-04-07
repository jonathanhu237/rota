import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react"

import { cn } from "@/lib/utils"

type ToastVariant = "default" | "destructive"

type Toast = {
  id: number
  title?: string
  description: string
  variant: ToastVariant
}

type ToastContextValue = {
  toast: (toast: Omit<Toast, "id">) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const nextID = useRef(1)

  const removeToast = useCallback((id: number) => {
    setToasts((currentToasts) =>
      currentToasts.filter((toast) => toast.id !== id),
    )
  }, [])

  const toast = useCallback(
    ({ variant, ...rest }: Omit<Toast, "id">) => {
      const id = nextID.current++

      setToasts((currentToasts) => [
        ...currentToasts,
        {
          id,
          variant,
          ...rest,
        },
      ])

      window.setTimeout(() => {
        removeToast(id)
      }, 4000)
    },
    [removeToast],
  )

  const value = useMemo(() => ({ toast }), [toast])

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="pointer-events-none fixed top-4 right-4 z-[60] flex w-[calc(100vw-2rem)] max-w-sm flex-col gap-2">
        {toasts.map((item) => (
          <div
            key={item.id}
            role="status"
            className={cn(
              "pointer-events-auto rounded-xl border bg-background p-4 shadow-lg",
              item.variant === "destructive"
                ? "border-destructive/30 bg-destructive/5"
                : "border-border",
            )}
          >
            {item.title && <div className="font-medium">{item.title}</div>}
            <div className="text-sm text-muted-foreground">
              {item.description}
            </div>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast() {
  const context = useContext(ToastContext)
  if (!context) {
    throw new Error("useToast must be used within a ToastProvider")
  }
  return context
}
