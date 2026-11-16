import { HighlightStyle, StreamLanguage, syntaxHighlighting } from '@codemirror/language'
import { tags } from '@lezer/highlight'

// 仅做常见 FANUC / Siemens 字词高亮，不执行或改写程序。
export const ncLanguage = StreamLanguage.define({
  startState: () => ({ commentDepth: 0 }),
  token(stream, state) {
    if (state.commentDepth || stream.peek() === '(') {
      while (!stream.eol()) {
        const c = stream.next()
        if (c === '(') state.commentDepth++
        if (c === ')' && --state.commentDepth === 0) break
      }
      return 'comment'
    }
    if (stream.eatSpace()) return null
    if (stream.match(/;.*/)) return 'comment'
    if (stream.match(/"[^"\n]*"/)) return 'string'
    if (stream.match(/\b(?:IF|THEN|ELSE|ENDIF|WHILE|DO|END|GOTO|GOTOF|GOTOB|PROC|RET|DEF)\b/i)) return 'keyword'
    if (stream.match(/\b[GM]\s*\d+(?:\.\d+)?/i)) return 'keyword'
    if (stream.match(/\b[NOTDH]\d+/i)) return 'labelName'
    if (stream.match(/\b[XYZABCUVWIJKRFSQPE]/i)) return 'propertyName'
    if (stream.match(/#(?:\d+|<[^>]*>)|\$[A-Za-z_][\w.]*/)) return 'variableName'
    if (stream.match(/[+-]?(?:\d+\.?\d*|\.\d+)/)) return 'number'
    if (stream.match(/[[\]*/=<>%+-]/)) return 'operator'
    stream.next()
    return null
  },
})

export const ncHighlight = syntaxHighlighting(HighlightStyle.define([
  { tag: tags.keyword, color: 'var(--nc-keyword)', fontWeight: 'bold' },
  { tag: tags.labelName, color: 'var(--nc-label)' },
  { tag: tags.propertyName, color: 'var(--nc-property)' },
  { tag: tags.number, color: 'var(--nc-number)' },
  { tag: tags.variableName, color: 'var(--nc-variable)' },
  { tag: tags.comment, color: 'var(--nc-comment)' },
  { tag: tags.string, color: 'var(--nc-string)' },
  { tag: tags.operator, color: 'var(--ui-body)' },
]))
