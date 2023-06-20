const DOMAIN = "intertube.download";

addEventListener('fetch', event => {
  event.respondWith(handleRequest(event.request))
})

/**
 * Respond to the request
 * @param {Request} request
 */
async function handleRequest(request) {
  console.log(request.url);

  var token = null;
  var redir = null;
  var query = (new URL(request.url)).searchParams;
  if (query) {
    token = query.get("token");
    redir = query.get("r");
    var dl = query.get("dl")
    if (dl) {
      redir = "/dl/" + query.get("dl");
    }
  }

  if (!token) {
    return new Response("no token", {status: 400});
  }

  
  var headers = new Headers(request.headers);
  headers.set('Access-Control-Allow-Origin', '*');
  headers.set("Set-Cookie", `tubetoken=${token}; Path=/; Domain=${DOMAIN}; HttpOnly; SameSite=None; Secure`);
  
  if (redir) {
    headers.set("Location", redir);
    return new Response("OK", {headers: headers, status: 303});
  }
  
  var ck = getCookie(request, "tubetoken");
  return new Response('cookie ' + ck, {headers: headers, status: 200})
}

function getCookie(request, name) {
  let result = ""
  const cookieString = request.headers.get("Cookie")
  if (cookieString) {
    const cookies = cookieString.split(";")
    cookies.forEach(cookie => {
      const cookieName = cookie.split("=")[0].trim()
      if (cookieName === name) {
        const cookieVal = cookie.split("=")[1]
        result = cookieVal
      }
    })
  }
  return result
}
