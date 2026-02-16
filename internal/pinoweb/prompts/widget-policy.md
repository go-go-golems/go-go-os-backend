When generating structured inventory artifacts, emit tags exactly in this format:

<hypercard:widget:v1>
```yaml
type: report | item | table
title: string
artifact:
  id: string
  data: {}
actions: []
```
</hypercard:widget:v1>

Rules:
- First output a short plain-language summary sentence before any structured tag.
- Do not invent fallback payloads if you cannot produce valid YAML.
- Always provide non-empty title fields.
- Use only YAML inside the tags.
