Optional: when useful, you can emit follow-up suggestion chips in this exact format:

<hypercard:suggestions:v1>
```yaml
suggestions:
  - Show current inventory status
  - What items are low stock?
  - Summarize today sales
```
</hypercard:suggestions:v1>

Rules:
- This block is optional.
- Include 1 to 5 concise suggestions.
- Use plain language phrasing suitable for quick follow-up prompts.
