# Temvia Rota rebuild baseline evidence

This snapshot fixes the comparison point for corrective fix attempt 1. It is
anchored to the immutable Git object rather than the mutable working tree.

- Baseline commit: `290be51287caa3831a3b8a14e196664964a2802d`
- Baseline commit tree: `91d4dc43af3815d01f24426a855e26a9756bbd82`
- Baseline subject: `chore: remove project agent skills`
- Captured: 2026-09-13 UTC

The following Git tree objects are the source inventories used by the parity
matrix and review. They can be rechecked without restoring or executing the
retired applications:

| Baseline area | Git tree object |
| --- | --- |
| `backend/` | `44982e44711513221bcf72bfa3641deeb90638fa` |
| `backend/internal/service/` | `43bcd88d117eb4fcca1133b29232e07e13c778ad` |
| `backend/internal/repository/` | `9d107e2fd83a33fa4bc993e1d47722e220517cff` |
| `frontend/` | `23e12a2d03854a79cae5632b36f380bdd6f195e0` |
| `frontend/src/routes/` | `217582ed78ca566b3190c9f781cea4353485fec7` |
| `frontend/src/components/` | `91572440653747fa1068db2a349e0ec578eef4bb` |
| `migrations/` | `ee2db23e874a9ebae7ec1424fa79ccf6bf83a2ea` |

Reproduction commands:

```sh
git rev-parse 290be51287caa3831a3b8a14e196664964a2802d^{tree}
git ls-tree -r --name-only 290be51287caa3831a3b8a14e196664964a2802d -- backend frontend migrations
git diff --check 290be51287caa3831a3b8a14e196664964a2802d --
```

No baseline object is modified by the rebuild. All current behavior is
implemented in `api/` and `admin/`; the baseline remains available through
its commit object for read-only comparison.
