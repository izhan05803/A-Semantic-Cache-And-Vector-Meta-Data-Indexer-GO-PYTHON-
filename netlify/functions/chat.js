const { GoogleGenerativeAI } = require('@google/generative-ai');

const genAI = new GoogleGenerativeAI(process.env.GOOGLE_API_KEY);

// In-memory cache — persists across warm invocations on Netlify
const cache = new Map();

function normalize(p) {
  return p.toLowerCase().replace(/[^a-z0-9\s]/g, '').replace(/\s+/g, ' ').trim();
}

exports.handler = async (event) => {
  const headers = {
    'Content-Type': 'application/json',
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Methods': 'POST, OPTIONS',
    'Access-Control-Allow-Headers': 'Content-Type',
  };

  if (event.httpMethod === 'OPTIONS') {
    return { statusCode: 204, headers, body: '' };
  }
  if (event.httpMethod !== 'POST') {
    return { statusCode: 405, headers, body: JSON.stringify({ error: 'only POST allowed' }) };
  }

  let body;
  try { body = JSON.parse(event.body); } catch {
    return { statusCode: 400, headers, body: JSON.stringify({ error: 'invalid JSON' }) };
  }
  if (!body.prompt) {
    return { statusCode: 400, headers, body: JSON.stringify({ error: 'prompt is required' }) };
  }

  const key = normalize(body.prompt);

  // Check cache
  if (cache.has(key)) {
    const entry = cache.get(key);
    return {
      statusCode: 200,
      headers,
      body: JSON.stringify({ response: entry.response, cached: true, score: 0.92 }),
    };
  }

  // Cache miss — call Gemini
  try {
    const model = genAI.getGenerativeModel({ model: 'gemini-flash-lite-latest' });
    const result = await model.generateContent(body.prompt);
    const response = result.response.text();

    // Store in cache
    cache.set(key, { response });

    return {
      statusCode: 200,
      headers,
      body: JSON.stringify({ response, cached: false, score: 0 }),
    };
  } catch (err) {
    return {
      statusCode: 502,
      headers,
      body: JSON.stringify({ error: 'Gemini call failed: ' + err.message }),
    };
  }
};
