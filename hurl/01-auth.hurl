# ETHOL - AUTHENTICATION

GET {{CAS_HOST}}/cas/login?service={{HOST}}/api/auth/cas-callback
HTTP 200

[Captures]
CAS_LT: xpath "string(//*[@name='lt']/@value)"

POST {{CAS_HOST}}/cas/login?service={{HOST}}/api/auth/cas-callback
[FormParams]
username: {{USERNAME}}
password: {{PASSWORD}}
lt: {{CAS_LT}}
_eventId: submit
submit: LOGIN
HTTP 302

[Captures]
CAS_TICKET: header "Location" regex "ticket=([^&]+)"

GET {{HOST}}/api/auth/cas-callback?ticket={{CAS_TICKET}}
HTTP *

GET {{HOST}}/api/auth/validasi-token
HTTP 200

[Asserts]
jsonpath "$.nomor" exists
jsonpath "$.nipnrp" exists
jsonpath "$.nama" exists
jsonpath "$.hakAkses" exists
