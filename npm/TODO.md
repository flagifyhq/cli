# npm Distribution — Pendientes

## Migrar a paquetes por plataforma (patrón esbuild/turbo)

El approach actual es un solo paquete `@flagify/cli` que descarga el binario en `postinstall`.
Para mayor robustez (CI restrictivos, sin network en postinstall), migrar a:

- `@flagify/cli` — paquete principal con `optionalDependencies`
- `@flagify/cli-darwin-arm64`
- `@flagify/cli-darwin-x64`
- `@flagify/cli-linux-arm64`
- `@flagify/cli-linux-x64`
- `@flagify/cli-win32-arm64`
- `@flagify/cli-win32-x64`

Cada paquete de plataforma contiene solo el binario. npm instala automáticamente solo el que corresponde al OS/arch.

Referencia: ver como lo hace [esbuild](https://github.com/evanw/esbuild/tree/main/npm) o [turbo](https://github.com/vercel/turbo/tree/main/packages).
