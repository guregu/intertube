'use strict';
const b2Domain = 'files.intertube.download'; // configure this as per instructions above
const b2Bucket = 'intertube'; // configure this as per instructions above

const b2UrlPath = `/file/${b2Bucket}/`;
addEventListener('fetch', event => {
	return event.respondWith(fileReq(event));
});

// define the file extensions we wish to add basic access control headers to
const imageTypes = ['png', 'jpg', 'gif', 'jpeg', 'webp', 'ico'];
const mimeFix = {
	// images
	png: "image/png",
	jpg: "image/jpeg",
	jpeg: "image/jpeg",
	gif: "image/gif",
	webp: "image/webp",
	ico: "image/x-icon",
	// audio
	mp3: "audio/mpeg",
	flac: "audio/flac",
};

// backblaze returns some additional headers that are useful for debugging, but unnecessary in production. We can remove these to save some size
const removeHeaders = [
	'x-bz-content-sha1',
	'x-bz-file-id',
	'x-bz-file-name',
	'x-bz-info-src_last_modified_millis',
	'X-Bz-Upload-Timestamp',
	'Expires'
];
const expiration = 31536000; // override browser cache for images - 1 year

const cacheSuffix = "v=2";

// define a function we can re-use to fix headers
const fixHeaders = function(url, status, headers){
	let newHdrs = new Headers(headers);

	let fixedMime = mimeFix[url.pathname.split('.').pop()];
	if (fixedMime) {
		newHdrs.set("Content-Type", fixedMime);
	}

	// add basic cors headers for images
	// if(corsFileTypes.includes(url.pathname.split('.').pop())){
		newHdrs.set('Access-Control-Allow-Origin', '*');
	// }
	// override browser cache for files when 200
	if(status === 200){
		newHdrs.set('Cache-Control', "public, max-age=" + expiration);
	}else{
		// only cache other things for 5 minutes
		// newHdrs.set('Cache-Control', 'public, max-age=300');
	}
	// set ETag for efficient caching where possible
	const ETag = newHdrs.get('x-bz-content-sha1') || newHdrs.get('x-bz-info-src_last_modified_millis') || newHdrs.get('x-bz-file-id');
	if(ETag){
		newHdrs.set('ETag', ETag);
	}
	// remove unnecessary headers
	removeHeaders.forEach(header => {
		newHdrs.delete(header);
	});
	return newHdrs;
};
async function fileReq(event){
	const cache = caches.default; // Cloudflare edge caching
	const url = new URL(event.request.url);

    let token = null;
	let picmode = false;

	if (url.pathname.startsWith("/pic/")) {
		let pickey = await TUBESPACE.get("pickey");
        if (pickey) {
            console.log("pickey", pickey);
            token = pickey;
			picmode = true;
        }
        url.pathname = b2UrlPath + url.pathname;
	} else {
        token = url.searchParams.get("token");
        if (!token) {
		    token = getCookie(event.request, "tubetoken");
	    }
    }
	
	if (!token || token == "") {
		return new Response("no token", {status: 403});
	}

	url.host = b2Domain;
	url.pathname = url.pathname.replace("/dl/", b2UrlPath);
	if(/*url.host === b2Domain && */!url.pathname.startsWith(b2UrlPath)){
		url.pathname = b2UrlPath + url.pathname;
	}

	let cacheReq = new Request(event.request);
	console.log(cacheReq.url.toString());

	let response = await cache.match(cacheReq); // try to find match for this request in the edge cache
	if(response){
		// use cache found on Cloudflare edge. Set X-Worker-Cache header for helpful debug
		let newHdrs = fixHeaders(url, response.status, response.headers);
		newHdrs.set('X-Worker-Cache', "true");
		return new Response(response.body, {
			status: response.status,
			statusText: response.statusText,
			headers: newHdrs
		});
	}
	// no cache, fetch image, apply Cloudflare lossless compression
	let authedHdr = new Headers(event.request.headers);
	authedHdr.set("Authorization", token);
	// url.pathname = "/file/intertube/" + url.pathname;
	
	response = await fetch(url, {headers: authedHdr, cf: {polish: "lossless"}});
	let newHdrs = fixHeaders(url, response.status, response.headers);
	newHdrs.set("X-Fresh", "true");
	// newHdrs.set("DEBUG-Token", token);

	if (response.status == 200 || response.status == 206) {
		newHdrs.set('Cache-Control', "public, max-age=" + expiration);
	}

	response = new Response(response.body, {
		status: response.status,
		statusText: response.statusText,
		headers: newHdrs
	});

	event.waitUntil(cache.put(cacheReq, response.clone()));
	return response;
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