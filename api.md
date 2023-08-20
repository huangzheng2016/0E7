# 0E7 API design document

## API

Usually used by client only or the secondary development of some applications.

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
| `cpu_use`    | double | yes      | cpu usage of the client,between 0-100                                 |
| `memory_use` | int    | yes      | memory usage of the client                                            |
| `memory_max` | int    | yes      | maximum memory of the client                                          |

#### Response

##### Content-Type

`application/json`

##### Response Parameters

| Parameter | Type     | Parent | Description                                                |
|-----------|----------|--------|------------------------------------------------------------|
| `message` | string   |        | operation status,including `success`,`fail`                |
| `error`   | string   |        | `fail` only,error message                                  |
| `sha256`  | []string |        | the sha256 values of the latest version of the application |

##### Response Status Codes

- 200: OK
- 400: Bad Request

#### Example

##### Request Example

```http
POST /api/heartbeat HTTP/1.1
Host: 0e7.cn

uuid=1ac5bb86-cda9-44b9-b7d5-acb59b498852&hostname=0e7&platform=windows&arch=amd64&cpu=12th%20Gen%20Intel(R)%20Core(TM)%20i7-12700KF&cpu_use=10.00&memory_use=1024&memory_max=4096
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
from the server through the `/api/update`.

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

Get the latest exploit tasks from server

#### Endpoint

`/api/exploit`

#### HTTP Method

`POST`

#### Request

##### Content-Type

`application/x-www-form-urlencoded`

##### Query Parameters

| Parameter  | Type   | Required | Description                                                           |
|------------|--------|----------|-----------------------------------------------------------------------|
| `uuid`     | string | yes      | UUID from config.ini,usually unique                                   |
| `platform` | string | yes      | platform of the client,including `windows`,`linux`,`darwin`,`freebsd` |
| `arch`     | string | yes      | arch of the client,including `386`,`amd64`,`arm64`                    |

#### Response

##### Content-Type

`application/json`

##### Response Parameters

| Parameter      | Type   | Parent | Description                                                                                                                      |
|----------------|--------|--------|----------------------------------------------------------------------------------------------------------------------------------|
| `message`      | string |        | operation status,including `success`,`fail`                                                                                      |
| `error`        | string |        | `fail` only,error message                                                                                                        |
| `exploit_uuid` | string |        | `exploit_uuid` of the exploit task,usually unique                                                                                |
| `filename`     | string |        | filename of the task,empty when file not exist                                                                                   |
| `environment`  | string |        | the running environment of the task                                                                                              |
| `command`      | string |        | the command that start the task                                                                                                  |
| `argv`         | string |        | the argv that start the task                                                                                                     |
| `flag`         | string |        | the flag format of the task,usually a regular expression. if not empty client will try to match output and return to `/api/flag` |

##### Response Status Codes

- 200: OK
- 202: No Task
- 400: Bad Request

#### Example

##### Request Example

```http
POST /api/exploit HTTP/1.1
Host: 0e7.cn

uuid=1ac5bb86-cda9-44b9-b7d5-acb59b498852&platform=windows&arch=amd64
```

##### Response Example

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "message":      "success",
    "error":        "",
    "exploit_uuid": "2ed62949-0825-4a6d-a3bf-26f782b07305",
    "filename":     "poc.py",
    "environment":  "auto_pipreqs=True;",
    "command":      "",
    "argv":         "",
    "flag":         "flag{.*}",
}
```

#### Notes

After successfully obtaining a task, the `time` column of the row corresponding to the task will be reduced one.

Only tasks whose `time` value is greater than 0 or equal to -2 (running forever) can be obtained through this interface

### exploit_download

Download the exploit task file from server

#### Endpoint

`/api/exploit_download`

#### HTTP Method

`POST`

#### Request

##### Content-Type

`application/x-www-form-urlencoded`

##### Query Parameters

| Parameter       | Type   | Required | Description                                                                                                        |
|-----------------|--------|----------|--------------------------------------------------------------------------------------------------------------------|
| `uexploit_uuid` | string | yes      | `exploit_uuid` of the exploit task,usually unique                                                                  |
| `filename`      | string | yes      | filename of the task,If you upload a tar or zip compressed file, you can specify the decompressed file to download |

#### Response

##### Content-Type

`application/octet-stream`

##### Response Parameters

| Parameter | Type   | Parent | Description                                 |
|-----------|--------|--------|---------------------------------------------|
| `message` | string |        | operation status,including `success`,`fail` |
| `error`   | string |        | error message                               |

##### Response Status Codes

- 200: OK
- 404: File Not Found

#### Example

##### Request Example

```http
POST /api/exploit_download HTTP/1.1
Host: 0e7.cn

exploit_uuid=2ed62949-0825-4a6d-a3bf-26f782b07305&filename=poc.py
```

##### Response Example

```http
HTTP/1.1 200 OK
Content-Type: application/octet-stream

Content-Disposition: attachment; filename=poc.py
<binary data>
```

#### Notes

It is not recommended to request the files in the compressed package, which may cause errors and is only for secondary
development

### exploit_output

Record the output of the exploit task

#### Endpoint

`/api/exploit_output`

#### HTTP Method

`POST`

#### Request

##### Content-Type

`application/x-www-form-urlencoded`

##### Query Parameters

| Parameter | Type   | Required | Description                                                          |
|-----------|--------|----------|----------------------------------------------------------------------|
| `id`      | int    | no       | the task unique id,if it is empty,it will return one for you         |
| `uuid`    | string | yes      | the `exploit_uuid` of the task                                       |
| `client`  | string | yes      | the `client_uuid` of the client that run the task                    |
| `output`  | string | yes      | arch of the client,including `386`,`amd64`,`arm64`                   |
| `status`  | string | yes      | the running status of the task,including `RUNNING`,`ERROR`,`SUCCESS` |

#### Response

##### Content-Type

`application/json`

##### Response Parameters

| Parameter | Type   | Parent | Description                                          |
|-----------|--------|--------|------------------------------------------------------|
| `message` | string |        | operation status,including `success`,`update`,`fail` |
| `error`   | string |        | error message                                        |
| `id`      | int    |        | the unique id for each task                          |

##### Response Status Codes

- 200: OK
- 400: Bad Request

#### Example

##### Request Example

```http
POST /api/exploit_output HTTP/1.1
Host: 0e7.cn

id=&uuid=2ed62949-0825-4a6d-a3bf-26f782b07305&client=1ac5bb86-cda9-44b9-b7d5-acb59b498852&output=flag{Hello,ZhengTai!}&status=SUCCESS
```

##### Response Example

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "message": "success",
    "error": "",
    "id": "7" 
}
```

#### Notes

if your exploit task `time` is 5, it will create 5 unique `id` for each subtask

if `id` is not empty and `status` is `RUNNING`,the `output` will be appended to the database and return `update` for the
live view of the task.

However, it is recommended to force an update with `SUCCESS` status

In secondary development, you can use not limited to the above three `status` to achieve more

### flag

Record the flag of the exploit task

#### Endpoint

`/api/flag`

#### HTTP Method

`POST`

#### Request

##### Content-Type

`application/x-www-form-urlencoded`

##### Query Parameters

| Parameter | Type   | Required | Description                                        |
|-----------|--------|----------|----------------------------------------------------|
| `uuid`    | string | yes      | the `exploit_uuid` of the task which find the flag |
| `flag`    | string | yes      | the flag which match the `flag` format             |

#### Response

##### Content-Type

`application/json`

##### Response Parameters

| Parameter | Type   | Parent | Description                                           |
|-----------|--------|--------|-------------------------------------------------------|
| `message` | string |        | operation status,including `success`,`skipped`,`fail` |
| `error`   | string |        | error message                                         |

##### Response Status Codes

- 200: OK
- 202: SKIPPED
- 400: Bad Request

#### Example

##### Request Example

```http
POST /api/flag HTTP/1.1
Host: 0e7.cn

uuid=2ed62949-0825-4a6d-a3bf-26f782b07305&flag=flag{Hello,ZhengTai!}
```

##### Response Example

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "message": "success",
    "error": ""
}
```

#### Notes

if sha256 not match the latest version and update in config.ini enabled, the client should download the latest version
from the server through the `/api/update` .

## WEBUI

Can be accessed by all applications, interface designed for the frontend

### exploit

Create a exploit task

#### Endpoint

`/webui/exploid`

#### HTTP Method

`POST`

#### Request

##### Content-Type

`application/x-www-form-urlencoded`

##### Query Parameters

| Parameter      | Type   | Required        | Description                                                                                                               |
|----------------|--------|-----------------|---------------------------------------------------------------------------------------------------------------------------|
| `exploit_uuid` | string | no              | automatically generated if empty, update all parameters if exist                                                          |
| `environment`  | string | no              | the running environment ot the task,format is `key1=value1;key2=value2;`,use in the `;` divide (include the end)          |
| `command`      | string | yes(or file)    | the command to start exploit task                                                                                         |
| `argv`         | string | no              | the argv to start exploit task                                                                                            |
| `platform`     | string | no              | the platform to run exploit task,format is `windows,linux,darwin,freebsd`,if empty means all,use in the middle `,` divide |
| `arch`         | string | no              | the arch to run exploit task,format is `386,amd64,arm64`,if empty means all,use in the middle `,` divide                  |
| `times`        | int    | no              | the number of times the script was run,if empty default -2(running forever),especially -1(stop)                           |
| `filter`       | string | no              | the filter to run exploit task,which `filter` match `client_id` ,if empty means all                                       |
| `file`         | file   | yes(or command) | the file that exploit run (or command only)                                                                               |

#### Response

##### Content-Type

`application/json`

##### Response Parameters

| Parameter      | Type   | Parent | Description                                 |
|----------------|--------|--------|---------------------------------------------|
| `message`      | string |        | operation status,including `success`,`fail` |
| `error`        | string |        | error message                               |
| `exploit_uuid` | string |        | the `exploit_uuid` of the task              |

##### Response Status Codes

- 200: OK
- 400: Bad Request
- 500: File System Error

#### Example

##### Request Example

```http
POST /webui/exploid HTTP/1.1
Host: 0e7.cn


------WebKitFormBoundarysxcf7YiPbFrA3rQm
Content-Disposition: form-data; name="exploit_uuid"


------WebKitFormBoundarysxcf7YiPbFrA3rQm
Content-Disposition: form-data; name="environment"

auto_pipreqs=True;
------WebKitFormBoundarysxcf7YiPbFrA3rQm

......(The command, argv, platform, arch, times, filter and other fields are omitted here, all are empty, refer to exploit_uuid)

------WebKitFormBoundarysxcf7YiPbFrA3rQm
Content-Disposition: form-data; name="file"; filename="poc.py"
Content-Type: text/x-python

<binary data>
------WebKitFormBoundarysxcf7YiPbFrA3rQm--

```

##### Response Example

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "message": "success",
    "error": "",
    "exploit_uuid": "2ed62949-0825-4a6d-a3bf-26f782b07305"
}
```

#### Notes

For the `exploit_id`,it will automatically generated if empty, update all parameters if exist

The update operation will not delete the original files, which may waste space, you can manually clear them

You can add `auto_pipreqs=True;` in environment to enable the automatic installation of python dependencies

`times` is the number of times the script was run,if empty default -2(running forever),especially -1(stop).It will be
automatically decremented by one each time it is run

### exploit_rename

Rename a exploit task

#### Endpoint

`/webui/exploit_rename`

#### HTTP Method

`POST`

#### Request

##### Content-Type

`application/x-www-form-urlencoded`

##### Query Parameters

| Parameter | Type   | Required | Description                  |
|-----------|--------|----------|------------------------------|
| `old`     | string | yes      | old exploit the exploit task |
| `new`     | string | yes      | new exploit the exploit task |

#### Response

##### Content-Type

`application/json`

##### Response Parameters

| Parameter    | Type   | Parent | Description                                        |
|--------------|--------|--------|----------------------------------------------------|
| `message`    | string |        | operation status,including `success`,`copy`,`fail` |
| `error`      | string |        | error message                                      |
| `exploit_id` | string |        | the new exploit id of the task                     |

##### Response Status Codes

- 200: OK
- 202: Copy Instead
- 400: Bad Request

#### Example

##### Request Example

```http
POST /webui/exploit_rename HTTP/1.1
Host: 0e7.cn

old=0a694cb2-2620-408b-acfb-83d00050ed85&new=2ed62949-0825-4a6d-a3bf-26f782b07306
```

##### Response Example

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "message": "success",
    "error": "",
    "exploit_uuid": "2ed62949-0825-4a6d-a3bf-26f782b07305"
}
```

#### Notes

If the original folder cannot be renamed, operation copy instead, but this will consume more space and you will need to
delete them manually

### exploit_show_output

show the output of the exploit task,including the live view

#### Endpoint

`/webui/exploit_show_output`

#### HTTP Method

`POST`

#### Request

##### Content-Type

`application/x-www-form-urlencoded`

##### Query Parameters

| Parameter      | Type   | Required | Description                                     |
|----------------|--------|----------|-------------------------------------------------|
| `id`           | int    | no       | if not empty,show the task match `id`           |
| `page_show`    | int    | no       | default 20,show `page_show` task a page         |
| `page_num`     | int    | no       | default 1,show the `page_num` page              |
| `exploit_uuid` | string | no       | if not empty,show the task match `exploit_uuid` |
| `client_uuid`  | string | no       | if not empty,show the task match `client_uuid`  |
| `platform`     | string | no       | if not empty,show the task match `platform`     |
| `arch`         | string | no       | if not empty,show the task match `arch`         |

#### Response

##### Content-Type

`application/json`

##### Response Parameters

| Parameter      | Type   | Parent   | Description                                                          |
|----------------|--------|----------|----------------------------------------------------------------------|
| `message`      | string |          | operation status,including `success`,`fail`                          |
| `error`        | string |          | error message                                                        |
| `page_count`   | int    |          | total number of pages                                                |
| `page_num`     | int    |          | the current pages                                                    |
| `page_show`    | int    |          | the number of the task one pages show                                |
| `result`       | object |          | a result object                                                      |
| `id`           | int    | `result` | the id of the task                                                   |                                          |
| `exploit_uuid` | string | `result` | the exploit_uuid of the task                                         |
| `client_uuid`  | string | `result` | the client_uuid of the client that run the task                      | 
| `output`       | string | `result` | the output of the task                                               |
| `status`       | string | `result` | the running status of the task,including `RUNNING`,`ERROR`,`SUCCESS` |

##### Response Status Codes

- 200: OK
- 400: Bad Request

#### Example

##### Request Example

```http
POST /webui/exploit_show_output HTTP/1.1
Host: 0e7.cn

```

##### Response Example

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "message": "success",
    "error": "",
    "page_count": 8,
    "page_num": 1,
    "page_show": 20,
    "result": [
        {
            "client_uuid": "1ac5bb86-cda9-44b9-b7d5-acb59b498852",
            "exploit_uuid": "2ed62949-0825-4a6d-a3bf-26f782b07305",
            "id": 147,
            "output": "Hello, ZhengTai!",
            "status": "SUCCESS"
        },
        .....(18 object omitted here)
        {
            "client_uuid": "1ac5bb86-cda9-44b9-b7d5-acb59b498852",
            "exploit_uuid": "2ed62949-0825-4a6d-a3bf-26f782b07305",
            "id": 128,
            "output": "Hello, ZhengTai!",
            "status": "SUCCESS"
        }
    ]
}
}
```

#### Notes

if not any parameters, it will show latest 20 tasks

`id` cannot take effect at the same time as `exploit_uuid`,`client_uuid`,`platform`,`arch`