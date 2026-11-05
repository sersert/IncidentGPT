import json, urllib.request, urllib.error, http.cookiejar, time
# Run only against the disposable compose preview, never against production.
base='http://127.0.0.1:8186'
jar=http.cookiejar.CookieJar(); client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
def call(path, method='GET', data=None, origin=None, expected=200, use_cookie=True):
 headers={}
 if origin: headers['Origin']=origin
 if data is not None: headers['Content-Type']='application/json'; data=json.dumps(data).encode()
 req=urllib.request.Request(base+path,data=data,headers=headers,method=method)
 try: response=(client if use_cookie else urllib.request.build_opener()).open(req,timeout=10)
 except urllib.error.HTTPError as e: response=e
 body=response.read()
 assert response.status==expected, (path,response.status,body[:120])
 assert response.headers.get('WWW-Authenticate') is None,(path,'browser auth challenge')
 return response,body
call('/');call('/healthz');call('/incidents/preview-deep-link')
for path in ['/api/v1/incidents','/api/v1/dashboard/stats','/api/v1/incidents/export.css','/ws/incidents','/ws/stream.js']:
 call(path,expected=401)
call('/_session_check',expected=404)
call('/api/v1/auth/session',expected=401)
call('/api/v1/auth/session','POST',{'username':'preview-operator','password':'incorrect'},base,401)
call('/api/v1/auth/session','POST',{'username':'preview-operator','password':'preview-only-password'},'http://evil.example',403)
call('/api/v1/auth/session','POST',{'username':'preview-operator','password':'preview-only-password'},base)
cookie=list(jar)[0]
assert cookie.has_nonstandard_attr('HttpOnly') and cookie.get_nonstandard_attr('SameSite')=='Strict'
call('/api/v1/auth/session');call('/api/v1/incidents');call('/api/v1/dashboard/stats')
call('/api/v1/auth/session','DELETE',origin='http://evil.example',expected=403)
call('/api/v1/auth/session')
call('/api/v1/auth/session','DELETE',origin=base,expected=204)
call('/api/v1/auth/session',expected=401);call('/api/v1/incidents',expected=401)
print('PASS: public sign-in/deep links/health; protected API + WS including static suffixes; bad password; Origin checks; session cookie; authenticated API; logout and revocation.')
