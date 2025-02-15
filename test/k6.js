import http from 'k6/http';
import { check } from 'k6';

export let options = {
    vus: 10,
    duration: '1s',
};

export default function () {
    const payload = JSON.stringify({
        url: 'http://example.com'
    });

    let res = http.post('http://localhost:8080/shorten', payload,{
        headers: { 'Content-Type': 'application/json' }
    });

    check(res, { 'status is 201': (r) => r.status === 201 });


    let shortCode;
    try {
        shortCode = res.json().short_code;
    } catch (e) {
        console.error('Failed to parse JSON:', e);
        return;
    }

	res = http.get(`http://localhost:8080/redirect?code=${shortCode}`);
	check(res, { 'status is 200': (r) => r.status === 200 });
}
