import { useEffect, useRef } from 'react'

type DictionaryEntryHtmlProps = {
  html: string
  className?: string
  onLookup?: (query: string) => Promise<void> | void
}

let activeAudio: HTMLAudioElement | null = null

export function DictionaryEntryHtml({ html, className, onLookup }: DictionaryEntryHtmlProps) {
  const rootRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    const root = rootRef.current
    if (!root) return

    const handleClick = (event: MouseEvent) => {
      void (async () => {
        const target = event.target
        if (!(target instanceof Element)) return
        const anchor = target.closest('a')
        if (!(anchor instanceof HTMLAnchorElement)) return

        const href = anchor.getAttribute('href')?.trim() ?? ''
        if (!href) return

        const lower = href.toLowerCase()

        if (lower.startsWith('entry://') || lower.startsWith('mdxentry://') || lower.startsWith('dict://')) {
          event.preventDefault()
          if (!onLookup) return
          const withoutScheme = href.includes('://') ? href.split('://', 2)[1] : href
          const nextQuery = withoutScheme.split('#', 1)[0]?.trim()
          if (!nextQuery) return
          await onLookup(nextQuery)
          return
        }

        const looksLikeAudio =
          lower.includes('/resource/snd:') ||
          lower.includes('/resource/sound:') ||
          lower.endsWith('.spx') ||
          lower.endsWith('.snd') ||
          lower.endsWith('.mp3') ||
          lower.endsWith('.wav') ||
          lower.endsWith('.ogg')

        if (!looksLikeAudio) return

        event.preventDefault()

        try {
          if (activeAudio) {
            activeAudio.pause()
            activeAudio.currentTime = 0
          }
          const audio = new Audio(anchor.href)
          activeAudio = audio
          await audio.play()
        } catch (error) {
          console.error('Failed to play dictionary audio resource', error)
        }
      })()
    }

    const handleMediaError = (event: Event) => {
      const target = event.target
      if (!(target instanceof HTMLAudioElement)) return
      target.classList.add('dictionary-audio-hidden')
      const wrapper = target.closest('audio')
      if (wrapper instanceof HTMLElement) {
        wrapper.classList.add('dictionary-audio-hidden')
      }
    }

    root.addEventListener('click', handleClick)
    root.addEventListener('error', handleMediaError, true)

    return () => {
      root.removeEventListener('click', handleClick)
      root.removeEventListener('error', handleMediaError, true)
    }
  }, [html, onLookup])

  return <div ref={rootRef} className={className} dangerouslySetInnerHTML={{ __html: html }} />
}
