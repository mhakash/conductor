import {
  autocompletion,
  type Completion,
  type CompletionContext,
  type CompletionResult,
} from '@codemirror/autocomplete'
import { EditorState, StateEffect, StateField, type Extension } from '@codemirror/state'
import {
  Decoration,
  EditorView,
  MatchDecorator,
  ViewPlugin,
  type DecorationSet,
  type ViewUpdate,
} from '@codemirror/view'
import { createContext, useContext } from 'react'

/** What an expression in the current step can refer to. */
export type ExpressionOption = {
  path: string
  detail: string
  kind: 'item' | 'step' | 'input' | 'secret' | 'run'
}

export const ExpressionScope = createContext<ExpressionOption[]>([])
export const useExpressionScope = () => useContext(ExpressionScope)

/** Cursor is inside an open `{{` with no `}}` after it on the same line. */
export function insideExpression(lineBefore: string): boolean {
  return lineBefore.lastIndexOf('{{') > lineBefore.lastIndexOf('}}')
}

/** Replaces the options offered inside `{{ }}`; dispatched when the scope changes. */
export const setExpressionScope = StateEffect.define<ExpressionOption[]>()

const scopeField = StateField.define<ExpressionOption[]>({
  create: () => [],
  update(value, tr) {
    for (const e of tr.effects) if (e.is(setExpressionScope)) return e.value
    return value
  },
})

const exprMark = Decoration.mark({ class: 'cm-expr' })
const exprMatcher = new MatchDecorator({
  regexp: /\{\{[^}]*\}\}/g,
  decoration: () => exprMark,
})

const highlightExpressions = ViewPlugin.fromClass(
  class {
    decorations: DecorationSet
    constructor(view: EditorView) {
      this.decorations = exprMatcher.createDeco(view)
    }
    update(u: ViewUpdate) {
      this.decorations = exprMatcher.updateDeco(u, this.decorations)
    }
  },
  { decorations: (v) => v.decorations },
)

// The loop item and upstream outputs are what you reach for most, so they lead the list.
const boostFor: Record<ExpressionOption['kind'], number> = {
  item: 4,
  step: 3,
  input: 2,
  secret: 1,
  run: 0,
}

const typeFor: Record<ExpressionOption['kind'], string> = {
  item: 'variable',
  step: 'property',
  input: 'variable',
  secret: 'constant',
  run: 'keyword',
}

function completeExpressions(bare: boolean) {
  return (ctx: CompletionContext): CompletionResult | null => {
    const line = ctx.state.doc.lineAt(ctx.pos)
    const before = line.text.slice(0, ctx.pos - line.from)
    if (!bare && !insideExpression(before)) return null
    const word = ctx.matchBefore(/[\w.]*/)
    const justOpened = bare ? before.trim() === '' : /\{\{\s*$/.test(before)
    if (!word || (word.from === word.to && !ctx.explicit && !justOpened)) {
      return null
    }
    const options: Completion[] = ctx.state.field(scopeField).map((o) => ({
      label: o.path,
      detail: o.detail,
      type: typeFor[o.kind],
      boost: boostFor[o.kind],
    }))
    return { from: word.from, options, validFor: /^[\w.]*$/ }
  }
}

const theme = EditorView.theme({
  '.cm-expr': {
    color: 'var(--color-brass)',
    backgroundColor: 'color-mix(in oklab, var(--color-brass) 10%, transparent)',
    borderRadius: '3px',
  },
  '.cm-tooltip.cm-tooltip-autocomplete': {
    backgroundColor: 'var(--color-raised)',
    border: '1px solid var(--color-line-strong)',
    borderRadius: '8px',
    overflow: 'hidden',
    boxShadow: '0 12px 32px rgb(0 0 0 / 0.45)',
  },
  '.cm-tooltip-autocomplete > ul': {
    fontFamily: 'var(--font-mono)',
    fontSize: '12px',
    maxHeight: '16em',
  },
  '.cm-tooltip-autocomplete > ul > li': { padding: '3px 10px', color: 'var(--color-fg)' },
  '.cm-tooltip-autocomplete > ul > li[aria-selected]': {
    backgroundColor: 'var(--color-hover)',
    color: 'var(--color-brass)',
  },
  '.cm-completionDetail': {
    fontFamily: 'var(--font-sans)',
    fontStyle: 'normal',
    color: 'var(--color-fg-subtle)',
    marginLeft: '12px',
  },
  '.cm-completionIcon': { display: 'none' },
  '.cm-completionMatchedText': { textDecoration: 'none', fontWeight: '600' },
})

/**
 * Highlighting and completion for `{{ … }}` in a CodeMirror field, or, with
 * bare set, for a field that holds a single expression without braces. Set
 * what completion offers by dispatching setExpressionScope.
 */
export function expressionSupport({ bare = false } = {}): Extension {
  return [
    scopeField,
    ...(bare ? [] : [highlightExpressions]),
    autocompletion({ override: [completeExpressions(bare)], icons: false, activateOnTyping: true }),
    theme,
  ]
}

/** Keeps a CodeMirror field to one line, like a text input. */
export const singleLine: Extension = EditorState.transactionFilter.of((tr) =>
  tr.newDoc.lines > 1 ? [] : tr,
)
