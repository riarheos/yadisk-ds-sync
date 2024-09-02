## Sync data between Synology NAS and Yandex Disk

This is a simple script to find differences and move your photos to and fro.

The config example (get your token at https://oauth.yandex.ru/authorize?response_type=token&client_id=e053600697f8401dbca06747f99d4904):
```
local:
  path: /volume1/photos
  ignore:
    - "@eaDir"
    - ".DS_Store"
    - "Thumbs.db"
remote:
  path: Photos
  token: y0_XXXXXXX
  workers: 5
  api_timeout: 30s
  download_timeout: 300s
```
