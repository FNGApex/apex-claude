// Lints the harness-neutral docs and skills.
//   1. Terminology: a model name, a Claude Code tool name or a `.claude/` path is allowed only on a line that says
//      claude, legacy or harness, or under a heading that does.
//   2. Reply style: the body of output-styles/apex.md equals the body of skills/reply-style/SKILL.md.
//   3. Line endings: no checked file holds CRLF.
// Usage: node scripts/check-terms.mjs [path ...]   (default: skills output-styles README.md docs/design)
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(fileURLToPath(new URL('..', import.meta.url)))
const DEFAULTS = ['skills', 'output-styles', 'README.md', 'docs/design']
const EXEMPT = new Set(['skills/apex-init/templates/tiers.md', 'skills/apex-init/templates/CLAUDE-section.md', 'MIGRATION.md'])
const TERMS = /\b(sonnet|opus|fable|mythos|haiku|ListAgents|SendMessage|Ultracode)\b|CLAUDE\.md|\.claude\/|~\/\.claude/
const EXCUSE = /claude|legacy|harness/i
const HEADING = /^#{1,4}\s/

const errors = []
const fail = (file, line, msg) => errors.push(`${file}${line ? `:${line}` : ''}: ${msg}`)
const rel = p => relative(root, p).split('\\').join('/')

function collect(p) {
  const st = statSync(p)
  if (!st.isDirectory()) return /\.(md|mjs)$/.test(p) ? [p] : []
  return readdirSync(p).sort().flatMap(n => collect(join(p, n)))
}

const given = process.argv.slice(2)
const files = []
for (const arg of given.length ? given : DEFAULTS) {
  const p = resolve(root, arg)
  // docs/design may not exist in a checkout that predates it; anything named explicitly must.
  if (!existsSync(p)) {
    if (given.length || arg !== 'docs/design') fail(arg, 0, 'does not exist')
    continue
  }
  files.push(...collect(p))
}

for (const file of [...new Set(files)]) {
  const name = rel(file)
  if (EXEMPT.has(name)) continue
  const text = readFileSync(file, 'utf8')
  if (text.includes('\r\n')) fail(name, 0, 'has CRLF line endings; use LF')
  let heading = '', fenced = false
  text.split('\n').forEach((line, i) => {
    if (name.endsWith('.md')) {
      if (/^\s*(```|~~~)/.test(line)) fenced = !fenced
      else if (!fenced && HEADING.test(line)) heading = line
    }
    if (!TERMS.test(line) || EXCUSE.test(line) || EXCUSE.test(heading)) return
    fail(name, i + 1, `"${line.match(TERMS)[0]}" outside a claude, legacy or harness context: ${line.trim()}`)
  })
}

const bodyOf = (file) => {
  const text = readFileSync(join(root, file), 'utf8')
  const m = /^---\n[\s\S]*?\n---\n/.exec(text)
  return m ? text.slice(m[0].length) : null
}
const [skill, style] = ['skills/reply-style/SKILL.md', 'output-styles/apex.md']
if (!existsSync(join(root, skill)) || !existsSync(join(root, style))) {
  fail(`${skill} / ${style}`, 0, 'both files must exist')
} else if (bodyOf(skill) === null || bodyOf(style) === null) {
  fail(`${skill} / ${style}`, 0, 'frontmatter is missing or unclosed')
} else if (bodyOf(skill) !== bodyOf(style)) {
  fail(style, 0, `body differs from ${skill}; the output style must repeat the skill's body byte for byte`)
}

if (errors.length) {
  console.error(errors.join('\n'))
  process.exit(1)
}
console.log('check-terms: ok')
