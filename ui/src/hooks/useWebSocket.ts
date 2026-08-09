import { useEffect, useRef, useState, useCallback } from 'react'
import type { WSMessage } from '@/api/types.ts'

const WS_URL = import.meta.env.VITE_WS_URL || `ws://${window.location.host}/ws/incidents`
const USE_MOCKS = import.meta.env.VITE_USE_MOCKS === 'true'

const RECONNECT_DELAYS = [1000, 2000, 4000, 8000, 16000, 30000]

export function useWebSocket(onMessage?: (msg: WSMessage) => void) {
  const [isConnected, setIsConnected] = useState(false)
  const [lastMessage, setLastMessage] = useState<WSMessage | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const retriesRef = useRef(0)
  const timerRef = useRef<ReturnType<typeof setTimeout>>()
  const onMessageRef = useRef(onMessage)
  onMessageRef.current = onMessage

  const connect = useCallback(() => {
    if (USE_MOCKS) {
      setIsConnected(true)
      return
    }

    try {
      const ws = new WebSocket(WS_URL)
      wsRef.current = ws

      ws.onopen = () => {
        setIsConnected(true)
        retriesRef.current = 0
      }

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data) as WSMessage
          setLastMessage(msg)
          onMessageRef.current?.(msg)
        } catch { /* ignore malformed */ }
      }

      ws.onclose = () => {
        setIsConnected(false)
        const delay = RECONNECT_DELAYS[Math.min(retriesRef.current, RECONNECT_DELAYS.length - 1)]
        retriesRef.current++
        timerRef.current = setTimeout(connect, delay)
      }

      ws.onerror = () => ws.close()
    } catch {
      setIsConnected(false)
    }
  }, [])

  useEffect(() => {
    connect()
    return () => {
      clearTimeout(timerRef.current)
      wsRef.current?.close()
    }
  }, [connect])

  return { isConnected, lastMessage }
}
