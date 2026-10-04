import { json } from '@codemirror/lang-json'
import { HighlightStyle, StreamLanguage, syntaxHighlighting } from '@codemirror/language'
import { shell } from '@codemirror/legacy-modes/mode/shell'
import { EditorView } from '@codemirror/view'
import { tags as t } from '@lezer/highlight'
import CodeMirror, { type Extension } from '@uiw/react-codemirror'
import { useEffect, useMemo, useRef } from 'react'

import {
  expressionSupport,
  setExpressionScope,
  singleLine as singleLineFilter,
  useExpressionScope,
} from './expressions'

const languages: Record<string, () => Extension> = {
  shell: () => StreamLanguage.define(shell),
  json: () => json(),
}

const theme = EditorView.theme(
  {
    '&': {
      fontSize: '12px',
      backgroundColor: 'var(--color-canvas)',
      color: 'var(--color-fg)',
      border: '1px solid var(--color-line)',
      borderRadius: '6px',
      transition: 'border-color 150ms',
    },
    '&:hover': { borderColor: 'var(--color-line-strong)' },
    '&.cm-focused': {
      outline: 'none',
      borderColor: 'color-mix(in oklab, var(--color-brass) 70%, transparent)',
    },
    '.cm-scroller': { fontFamily: 'var(--font-mono)', lineHeight: '1.6' },
    '.cm-content': { padding: '8px 0', caretColor: 'var(--color-brass)' },
    '.cm-line': { padding: '0 10px' },
    '.cm-gutters': { display: 'none' },
    '.cm-activeLine': { backgroundColor: 'transparent' },
    '.cm-cursor': { borderLeftColor: 'var(--color-brass)' },
    '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection': {
      backgroundColor: 'color-mix(in oklab, var(--color-brass) 28%, transparent) !important',
    },
    '.cm-placeholder': { color: 'var(--color-fg-subtle)' },
  },
  { dark: true },
)

const highlight = syntaxHighlighting(
  HighlightStyle.define([
    { tag: [t.keyword, t.operatorKeyword, t.controlKeyword], color: 'var(--color-brass)' },
    { tag: [t.string, t.special(t.string)], color: 'var(--color-code-string)' },
    { tag: [t.number, t.bool, t.null], color: 'var(--color-code-number)' },
    { tag: [t.variableName, t.propertyName, t.attributeName], color: 'var(--color-code-variable)' },
    { tag: t.comment, color: 'var(--color-fg-subtle)', fontStyle: 'italic' },
  ]),
)

const singleLineTheme = EditorView.theme({
  '.cm-content': { padding: '6px 0' },
  '.cm-scroller': { overflow: 'hidden' },
})

// An expression field reads as code throughout, like the inside of {{ }}.
const expressionTheme = EditorView.theme({ '.cm-content': { color: 'var(--color-brass)' } })

type CodeFieldProps = {
  id: string
  value: string
  onChange: (value: string) => void
  language?: string
  placeholder?: string
  /** Behave like a one-line text input (no wrapping, Enter does nothing). */
  singleLine?: boolean
  /** The whole value is one expression, written without {{ }}. Implies singleLine. */
  expression?: boolean
}

/**
 * A CodeMirror field that understands `{{ … }}` expressions: they are
 * highlighted, and typing inside one offers what the step can refer to.
 */
export function CodeField({
  id,
  value,
  onChange,
  language,
  placeholder,
  singleLine,
  expression = false,
}: CodeFieldProps) {
  const oneLine = singleLine || expression
  const scope = useExpressionScope()
  const view = useRef<EditorView | null>(null)
  useEffect(() => {
    view.current?.dispatch({ effects: setExpressionScope.of(scope) })
  }, [scope])

  // Stable across renders so CodeMirror doesn't reconfigure on every keystroke.
  const extensions = useMemo(() => {
    const lang = language ? languages[language] : undefined
    return [
      theme,
      highlight,
      expressionSupport({ bare: expression }),
      ...(oneLine ? [singleLineFilter, singleLineTheme] : [EditorView.lineWrapping]),
      ...(expression ? [expressionTheme] : []),
      ...(lang ? [lang()] : []),
    ]
  }, [language, oneLine, expression])

  return (
    <CodeMirror
      id={id}
      value={value}
      onChange={onChange}
      onCreateEditor={(v) => {
        view.current = v
        v.dispatch({ effects: setExpressionScope.of(scope) })
      }}
      placeholder={placeholder}
      theme="none"
      minHeight={oneLine ? undefined : '72px'}
      maxHeight={oneLine ? undefined : '320px'}
      basicSetup={{
        lineNumbers: false,
        foldGutter: false,
        highlightActiveLine: false,
        // Ours replaces the default word completion (see expressions.ts).
        autocompletion: false,
      }}
      extensions={extensions}
    />
  )
}
