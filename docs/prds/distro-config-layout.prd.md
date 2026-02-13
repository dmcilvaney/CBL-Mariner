# PRD: Distro Configuration Layout

Title: PRD: Distro Configuration Layout
Author: Daniel McIlvaney
Created: 2026-02-13
Status: Draft

## Overview

This PRD defines the layout and conventions for distro-level configuration files in the `distro/` directory. These files control build defaults, upstream references, and mock build environments shared across all projects.

## Problem Statement

Distro configuration is foundational — changes in `distro/` affect every component build. Without clear layout conventions, contributors may:

- Add distro-specific config in the wrong location (e.g., project-level instead of distro-level)
- Create ad-hoc distro TOML files without following naming conventions
- Modify mock configs without understanding the template system
- Duplicate build defaults that should be inherited from the distro definition

## Scope

### In Scope (v1)

1. **Directory structure** — `distro/` is the canonical location for all distro-wide configuration
2. **File naming** — `*.distro.toml` naming convention for distro definitions
3. **Include hierarchy** — `distro.toml` includes all `*.distro.toml` files; this is stitched into the global config via `azldev.toml`
4. **Mock configs** — `mock/` subdirectory contains mock build environment configs (`.cfg` files and `.tpl` templates)
5. **Build defaults** — Each distro definition declares default build defines (`dist`, `vendor`, `distribution`, etc.) inherited by all components

### Out of Scope (v1)

- Adding new distro targets (e.g., RHEL, SUSE)
- Mock config generation or templating tooling
- Cross-project distro config sharing mechanisms

## Technical Design

### Required Layout

```text
distro/
├── AGENTS.md                        # Agent guidance for distro config
├── README.md                        # Human-readable documentation
├── distro.toml                      # Root include — includes all *.distro.toml
├── azurelinux.distro.toml           # Azure Linux distro definition
├── fedora.distro.toml               # Fedora upstream definition
└── mock/                            # Mock build environment configs
    ├── azurelinux-4.0-x86_64.cfg
    ├── azurelinux-4.0-aarch64.cfg
    └── azurelinux-4.0.tpl           # Shared template for mock configs
```

### Conventions

1. **One distro definition per file** — Each `*.distro.toml` defines exactly one distro target
2. **File naming** — `<distro-name>.distro.toml` (lowercase, hyphen-separated)
3. **Include file** — `distro.toml` must include all `*.distro.toml` files; do not add distro definitions elsewhere
4. **Mock config naming** — `<distro>-<version>-<arch>.cfg` for arch-specific configs
5. **No project-level overrides** — Build defaults belong in distro definitions, not in `project.toml` or component files (components may override individual defines via `build.defines`)
6. **Documentation** — Both `AGENTS.md` (for AI agents) and `README.md` (for humans) should be maintained in `distro/`

### Build Default Inheritance

Distro definitions declare default build defines that flow to all components:

```
azldev.toml → distro/distro.toml → distro/azurelinux.distro.toml (defaults)
                                          ↓
                                   base/comps/<name>.comp.toml (overrides via build.defines)
```

Components should only override distro defaults when they have a specific reason to diverge.
