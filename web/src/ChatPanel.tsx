import {
  ArrowUp,
  Bot,
  Check,
  ChevronRight,
  CornerDownLeft,
  MapPin,
  MessageSquare,
  Plus,
  Sparkles,
  Square,
} from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import type { ChatMessage, Health } from './types'

type Props = {
  health: Health | null
  messages: ChatMessage[]
  busy: boolean
  onSend: (text: string, context: boolean) => void
  onCancel: () => void
  onReset: () => void
}
const suggestions = [
  '查找地图范围内的咖啡店',
  '计算两个 WGS84 坐标的距离',
  '判断一个坐标是否落在围栏内',
]
export default function ChatPanel({ health, messages, busy, onSend, onCancel, onReset }: Props) {
  const [message, setMessage] = useState(''),
    [includeContext, setIncludeContext] = useState(true)
  const bottom = useRef<HTMLDivElement>(null)
  useEffect(() => {
    bottom.current?.scrollIntoView?.({ behavior: 'smooth', block: 'nearest' })
  }, [messages, busy])
  const enabled = health?.agent_enabled === true
  function send() {
    if (!message.trim() || busy || !enabled) return
    onSend(message.trim(), includeContext)
    setMessage('')
  }
  return (
    <aside className="chat-panel panel">
      <div className="panel-heading">
        <span className="eyebrow">SPATIAL COPILOT</span>
        <button
          className="icon-button"
          aria-label="清空对话"
          title="清空本页对话"
          disabled={busy}
          onClick={onReset}
        >
          <Plus size={17} />
        </button>
      </div>
      <div className="chat-body">
        {!messages.length ? (
          <div className="chat-welcome">
            <div className="assistant-mark">
              <Sparkles size={23} />
            </div>
            <span className="eyebrow">从一个问题开始</span>
            <h2>
              把空间问题，
              <br />
              变成确定的答案。
            </h2>
            <p>
              描述你想探索的区域，
              <br />让 Agent 调用空间工具完成计算。
            </p>
            <div className="suggestions">
              {suggestions.map((text, i) => (
                <button key={text} onClick={() => setMessage(text)}>
                  <span>
                    {i === 0 ? (
                      <MapPin size={15} />
                    ) : i === 1 ? (
                      <Sparkles size={15} />
                    ) : (
                      <MessageSquare size={15} />
                    )}
                    {text}
                  </span>
                  <ChevronRight size={14} />
                </button>
              ))}
            </div>
            <div className="capability-note">
              <Check size={13} />
              受控工具调用<span>·</span>结构化结果
            </div>
          </div>
        ) : (
          <div className="messages">
            {messages.map((m) => (
              <div key={m.id} className={'message ' + m.role + (m.error ? ' failed' : '')}>
                <div className="message-role">
                  {m.role === 'assistant' ? (
                    <Bot size={14} />
                  ) : (
                    <span className="user-avatar">U</span>
                  )}
                  {m.role === 'assistant' ? 'PulseGeo' : '你'}
                </div>
                <p>{m.content}</p>
              </div>
            ))}
            {busy && (
              <div className="thinking">
                <span className="live-dot" />
                正在调用空间工具…
              </div>
            )}
            <div ref={bottom} />
          </div>
        )}
      </div>
      {!enabled && (
        <div className="agent-notice">
          <span className="status-dot amber" />
          <div>
            <strong>{health ? 'Agent 尚未配置' : '正在连接服务'}</strong>
            <p>
              {health ? '配置模型后可对话；地图检索现在就能用。' : '正在检查模型与数据源状态。'}
            </p>
          </div>
        </div>
      )}
      <form
        className="chat-composer"
        onSubmit={(e) => {
          e.preventDefault()
          send()
        }}
      >
        <label className="context-toggle">
          <input
            type="checkbox"
            checked={includeContext}
            onChange={(e) => setIncludeContext(e.target.checked)}
          />
          <MapPin size={13} />
          附带当前地图范围<span>WGS84</span>
        </label>
        <textarea
          aria-label="空间任务"
          value={message}
          maxLength={3500}
          onChange={(e) => setMessage(e.target.value)}
          placeholder="描述一个空间任务…"
          rows={3}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
              e.preventDefault()
              send()
            }
          }}
        />
        <div className="composer-bottom">
          <span>
            <CornerDownLeft size={11} />
            发送 · Shift Enter 换行
          </span>
          {busy ? (
            <button className="send-button" type="button" aria-label="停止请求" onClick={onCancel}>
              <Square size={14} />
            </button>
          ) : (
            <button
              className="send-button"
              type="submit"
              aria-label="发送消息"
              disabled={!enabled || !message.trim()}
            >
              <ArrowUp size={18} />
            </button>
          )}
        </div>
      </form>
      <div className="chat-disclaimer">独立单次任务 · 请核对右侧工具计算结果</div>
    </aside>
  )
}
