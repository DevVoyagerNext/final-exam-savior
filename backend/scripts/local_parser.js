const fs = require('fs');
const path = require('path');

function usage() {
  console.error('Usage: node scripts/local_parser.js <filePath> <sourceType>');
  process.exit(2);
}

function looksLikePDF(filePath, sourceType) {
  const ext = path.extname(filePath).toLowerCase();
  return ext === '.pdf' || String(sourceType || '').toLowerCase().includes('application/pdf');
}

async function extractPDF(buffer) {
  const pdfParse = require('pdf-parse');
  const result = await pdfParse(buffer);
  return (result && typeof result.text === 'string' ? result.text : '').trim();
}

async function extractOffice(buffer) {
  const docstream = require('@jose.espana/docstream');
  const ast = await docstream.parseOffice(buffer);
  const text = ast && typeof ast.toText === 'function' ? ast.toText() : '';
  return String(text || '').trim();
}

async function main() {
  const filePath = process.argv[2];
  const sourceType = process.argv[3] || '';
  if (!filePath) {
    usage();
  }

  const buffer = fs.readFileSync(filePath);
  const text = looksLikePDF(filePath, sourceType)
    ? await extractPDF(buffer)
    : await extractOffice(buffer);

  process.stdout.write(text);
}

main().catch((err) => {
  const message = err && err.stack ? err.stack : String(err);
  process.stderr.write(message);
  process.exit(1);
});
