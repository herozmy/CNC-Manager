import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import { highlightSelectionMatches, openSearchPanel, search, searchKeymap } from '@codemirror/search'
import { Compartment, EditorState } from '@codemirror/state'
import { drawSelection, EditorView, highlightActiveLine, keymap, lineNumbers } from '@codemirror/view'
import { useEffect, useImperativeHandle, useRef, useState } from 'react'
import type { Ref } from 'react'
import { ncHighlight, ncLanguage } from '../lib/nc-language'
import { Button } from './ui/Button'

export interface NCCodeEditorHandle { getValue: () => string; focus: () => void }
export interface NCCodeEditorProps {
  initialValue: string
  label?: string
  readOnly: boolean
  onDirtyChange: (dirty: boolean) => void
  ref?: Ref<NCCodeEditorHandle>
}

export default function NCCodeEditor({ initialValue, label = 'NC 程序内容', readOnly, onDirtyChange, ref }: NCCodeEditorProps) {
  const host = useRef<HTMLDivElement>(null)
  const view = useRef<EditorView | null>(null)
  const access = useRef(new Compartment())
  const [line, setLine] = useState('')
  const [error, setError] = useState('')
  const [position, setPosition] = useState('1:1')
  const [lineCount, setLineCount] = useState(1)
  useImperativeHandle(ref, () => ({ focus: () => view.current?.focus(), getValue: () => view.current?.state.doc.toString() ?? initialValue }), [initialValue])

  useEffect(() => {
    if (!host.current) return
    const state = EditorState.create({ doc: initialValue, extensions: [
      lineNumbers(), history(), drawSelection(), highlightActiveLine(), highlightSelectionMatches(),
      ncLanguage, ncHighlight, search({ top: true }),
      keymap.of([...defaultKeymap, ...historyKeymap, ...searchKeymap]),
      access.current.of([EditorState.readOnly.of(true), EditorView.editable.of(false)]),
      EditorView.contentAttributes.of({ 'aria-label': label, 'aria-multiline': 'true', spellcheck: 'false', autocapitalize: 'off', autocorrect: 'off', tabindex: '0' }),
      EditorState.phrases.of({ 'Find': '查找', 'Replace': '替换', 'next': '下一个', 'previous': '上一个', 'all': '全部', 'match case': '区分大小写', 'regexp': '正则表达式', 'by word': '全词匹配', 'replace': '替换', 'replace all': '全部替换', 'close': '关闭查找' }),
      EditorView.theme({
        '&': { fontSize: '14px', color: 'var(--ui-ink)', backgroundColor: 'var(--ui-canvas)', borderRadius: '0.375rem', border: '1px solid var(--ui-line-strong)', overflow: 'hidden' },
        '&.cm-focused': { outline: '2px solid var(--ui-accent)' },
        '.cm-scroller': { overflow: 'auto', height: 'min(48dvh, 520px)', minHeight: '220px', fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' },
        '.cm-content': { padding: '8px 0', caretColor: 'var(--ui-accent)' },
        '.cm-panels': { backgroundColor: 'var(--ui-muted)', color: 'var(--ui-ink)' },
        '.cm-activeLineGutter': { backgroundColor: 'var(--ui-accent-soft)', color: 'var(--ui-accent)' },
        '.cm-gutters': { backgroundColor: 'var(--ui-muted)', color: 'var(--ui-subtle)', borderRight: '1px solid var(--ui-line)' },
        '.cm-activeLine': { backgroundColor: 'var(--ui-accent-soft)' },
        '.cm-search': { display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '6px', padding: '8px', paddingRight: '28px' },
        '.cm-search input:not([type=checkbox])': { maxWidth: '100%', minHeight: '36px', border: '1px solid var(--ui-line-strong)', borderRadius: '4px' },
        '.cm-search label': { display: 'inline-flex', alignItems: 'center', gap: '4px', minHeight: '32px', whiteSpace: 'nowrap' },
        '.cm-search button': { minHeight: '36px', background: 'var(--ui-surface)', color: 'var(--ui-ink)', border: '1px solid var(--ui-line-strong)', borderRadius: '6px', padding: '4px 8px' },
      }),
      EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          onDirtyChange(!update.state.doc.eq(state.doc))
          setLineCount(update.state.doc.lines)
        }
        if (update.docChanged || update.selectionSet) {
          const head = update.state.selection.main.head
          const at = update.state.doc.lineAt(head)
          setPosition(`${at.number}:${head - at.from + 1}`)
        }
      }),
    ] })
    const editor = new EditorView({ state, parent: host.current })
    view.current = editor
    setLineCount(state.doc.lines)
    return () => { editor.destroy(); view.current = null }
  }, [initialValue, label, onDirtyChange])

  useEffect(() => {
    view.current?.dispatch({ effects: access.current.reconfigure([
      EditorState.readOnly.of(readOnly), EditorView.editable.of(!readOnly),
      EditorView.contentAttributes.of({ 'aria-readonly': String(readOnly) }),
    ]) })
  }, [readOnly, initialValue, onDirtyChange])

  const jump = () => {
    const editor = view.current
    const n = Number(line)
    if (!editor || !Number.isInteger(n) || n < 1 || n > editor.state.doc.lines) {
      setError(`请输入 1~${lineCount} 的行号`)
      return
    }
    setError('')
    const from = editor.state.doc.line(n).from
    editor.dispatch({ selection: { anchor: from }, effects: EditorView.scrollIntoView(from, { y: 'center' }) })
    editor.focus()
  }
  return <div className="min-w-0 space-y-2">
    <div className="flex flex-wrap items-center gap-2">
      <Button variant="secondary" onClick={() => view.current && openSearchPanel(view.current)}>查找 / 替换</Button>
      <input aria-label="跳转行号" inputMode="numeric" value={line} onChange={(e) => setLine(e.target.value)} onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); jump() } }} placeholder="行号" className="h-12 w-20 rounded-md border border-line-strong px-3" />
      <Button variant="secondary" onClick={jump}>跳转</Button>
      <span className="text-xs text-muted">行:列 {position} · 共 {lineCount} 行</span>
    </div>
    {error && <p role="alert" className="text-sm text-danger">{error}</p>}
    <div ref={host} className="min-w-0" />
  </div>
}
