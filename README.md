# Filestash with proxy-header auth plugin

This repo is **Filestash** with one extra plugin compiled in: **proxy-header authentication**. You put Filestash behind a forward-auth proxy (e.g. [Tinyauth](https://github.com/nicksherron/tinyauth)); the proxy sets a header with the username (e.g. `Remote-User`). The plugin reads that header—if it’s missing you get 401, if present you’re logged in as that user. No login form.

**Why compiled?** Filestash (currently?) only supports auth via compiled plugins, not drop-in runtime plugins, so this image is a custom build.

---

## Use the image

Pull and run:

```bash
docker pull rynomarree/filestash-authenticate-proxy:latest
```

In your compose use `image: rynomarree/filestash-authenticate-proxy:latest` (or build locally, see below).

**In Filestash:** Admin → Authentication → **Proxy header**, set **Header name** to whatever your proxy sends (default `Remote-User`).

---

## Backend (e.g. OMV Samba) and attribute mapping

For backends that require login (e.g. Samba, SFTP) you always set the **password** in Filestash—the plugin only provides the **username** (from the header):

- **Admin → Backend → Attribute mapping:** **Username** = `{{ .user }}`, **Password** = the shared backend password (one literal for this backend).

---

## Build from source

Dockerfile in this repo is upstream Filestash with the plugin injected.

```bash
docker build -t filestash-with-proxy-auth -f Dockerfile .
```

To refresh the Dockerfile from upstream Filestash and then build: `./build.sh` (or `./build.sh -g` to only regenerate the Dockerfile). Build can take several minutes.

---

## Licence

Filestash is AGPL-3.0. This plugin is for use with your own build.
