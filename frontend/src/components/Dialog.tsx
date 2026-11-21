import { useLayoutEffect, useRef, type ReactNode } from 'react'

interface DialogProps {
  className: string
  label: string
  onDismiss: () => void
  children: ReactNode
}

export function Dialog({ className, label, onDismiss, children }: DialogProps) {
  const ref = useRef<HTMLDialogElement>(null)
  const pressedBackdrop = useRef(false)

  useLayoutEffect(() => {
    const dialog = ref.current!
    dialog.showModal()
    dialog.querySelector<HTMLElement>('[data-initial-focus]')?.focus({ preventScroll: true })
    return () => dialog.close()
  }, [])

  function isBackdrop(event: React.MouseEvent<HTMLDialogElement>) {
    if (event.target !== event.currentTarget) return false
    const { left, right, top, bottom } = event.currentTarget.getBoundingClientRect()
    return event.clientX < left || event.clientX > right || event.clientY < top || event.clientY > bottom
  }

  return (
    <dialog
      ref={ref}
      className={className}
      aria-label={label}
      onCancel={(event) => { event.preventDefault(); onDismiss() }}
      onPointerDown={(event) => { pressedBackdrop.current = isBackdrop(event) }}
      onClick={(event) => { if (pressedBackdrop.current && isBackdrop(event)) onDismiss() }}
    >
      {children}
    </dialog>
  )
}
