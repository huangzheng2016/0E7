# 0E7 API design document

## API

Usually used by Client only.

### heartbeat

Initiate a heartbeat request to the server

#### Endpoint

`/api/heartbeat`

#### HTTP Method

`POST`

#### Request

##### Content-Type

`application/x-www-form-urlencoded`

##### Query Parameters

| Parameter    | Type   | Required | Description                                                           |
|--------------|--------|----------|-----------------------------------------------------------------------|
| `uuid`       | string | yes      | UUID from config.ini,usually unique                                   |
| `hostname`   | string | yes      | hostname of the client                                                |
| `platform`   | string | yes      | platform of the client,including `windows`,`linux`,`darwin`,`freebsd` |
| `arch`       | string | yes      | arch of the client,including `386`,`amd64`,`arm64`                    |
| `cpu`        | string | yes      | cpuname of the client                                                 |
| `cpu_use`    | string | yes      | cpu usage of the client,a number in string format, between 0-100      |
| `memory_use` | string | yes      | memory usage of the client,a number in string format                  |
| `memory_max` | string | yes      | maximum memory of the client,a number in string format                |

#### Response

##### Content-Type

`application/json`

##### Response Parameters

| Parameter | Type     | Parent | Description                                                |
|-----------|----------|--------|------------------------------------------------------------|
| `message` | string   |        | operation status,including `success`,`fail`                |
| `error`   | string   |        | error message,`fail` only                                  |
| `sha256`  | []string |        | the sha256 values of the latest version of the application |

##### Response Status Codes

- 200: OK
- 400: Bad Request

#### Example

##### Request Example

```http
POST /api/heartbeat HTTP/1.1
Host: 0e7.cn

uuid=0e7&hostname=hz2016-0e7&platform=windows&arch=amd64&cpu=12th%20Gen%20Intel(R)%20Core(TM)%20i7-12700KF&cpu_use=10.00&memory_use=1024&memory_max=4096
```

##### Response Example

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "message": "success",
    "sha256": [
        "dbcb110bec8316f5fc30cf3534fbed29548298c3b160a6ffd36b97de5af51afe",
        "21420fb38b092ddc93bf09ebba2564a38806bae9598c9857afffeff48f190232",
        "69f8b3066064091acc0d85c0879a0ee3c28ccca080e5f2e6043274d801643b65",
        "f2df7480d7fe2d385066b2f0fbc72fcbfaa9543229330e1392b2d6d511710b86",
        "bb8bce4c4e2bd405efbe5001e51d29b467defc972e20b10697e31c5f949ae1f3",
        "3a8766c8352732d57e30253fa858f1cfca76304d9826992706863f9627bd5db0",
        "4175d6b228d2f955a4e481b097e8a3305090856b3a9beb74444fc2ae28ae7712",
        "6299d75e4c0c62bc804eb09dc9125d93cf6d1a2554634d5d4b3bb6aab254ca99",
        "0452ca0d99e7aa671caf61e7866bda8e436e00967b1ce00d68946edc7f9f5593"
    ]
}
```

#### Notes

if sha256 not match the latest version and update in config.ini enabled, the client should download the latest version
from the server through the `/api/update` .

### update

Download the latest version from server

#### Endpoint

`/api/update`

#### HTTP Method

`POST`

#### Request

##### Content-Type

`application/x-www-form-urlencoded`

##### Query Parameters

| Parameter  | Type   | Required | Description                                                           |
|------------|--------|----------|-----------------------------------------------------------------------|
| `platform` | string | yes      | platform of the client,including `windows`,`linux`,`darwin`,`freebsd` |
| `arch`     | string | yes      | arch of the client,including `386`,`amd64`,`arm64`                    |

#### Response

##### Content-Type

`application/octet-stream`

##### Response Parameters

| Parameter  | Type   | Parent | Description                               |
|------------|--------|--------|-------------------------------------------|
| `filename` | string |        | format like `0e7_[platform]_[arch](.exe)` |

##### Response Status Codes

- 200: OK
- 404: File Not Found

#### Example

##### Request Example

```http
POST /api/update HTTP/1.1
Host: 0e7.cn

platform=windows&arch=amd64
```

##### Response Example

```http
HTTP/1.1 200 OK
Content-Type: application/octet-stream
Content-Disposition: attachment; filename=0e7_windows_amd64.exe

<binary data>
```

### exploit

### exploit_download

### exploit_output

### flag

## WEBUI
