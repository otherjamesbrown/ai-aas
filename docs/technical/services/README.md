# Services Directory

Each service resides under `services/<name>/` and must:

- Include the shared automation template (`include ../../templates/service.mk`)
- Provide service-specific README detailing purpose and ownership
- Keep code within Go modules referenced by `go.work`

See `docs/services/customizing.md` and `docs/services/checklist.md` for guidance.

