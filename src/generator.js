import QRCode from 'qrcode';

document.addEventListener('DOMContentLoaded', () => {
  const textInput = document.getElementById('qr-text');
  const sizeInput = document.getElementById('qr-size');
  const errorCorrectionSelect = document.getElementById('error-correction');
  const generateBtn = document.getElementById('generate-btn');
  const qrResult = document.getElementById('qr-result');
  const qrCanvas = document.getElementById('qr-canvas');
  const downloadLink = document.getElementById('download-link');

  // Check for URL parameters to auto-populate
  const urlParams = new URLSearchParams(window.location.search);
  const textParam = urlParams.get('text');
  const sizeParam = urlParams.get('size');
  const ecParam = urlParams.get('ec');

  if (textParam) {
    textInput.value = textParam;
  }
  if (sizeParam && !isNaN(parseInt(sizeParam))) {
    sizeInput.value = parseInt(sizeParam);
  }
  if (ecParam && ['L', 'M', 'Q', 'H'].includes(ecParam.toUpperCase())) {
    errorCorrectionSelect.value = ecParam.toUpperCase();
  }

  // Auto-generate if text is provided via URL
  if (textParam) {
    generateQRCode();
  }

  generateBtn.addEventListener('click', generateQRCode);

  // Also generate on Enter key in text input
  textInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && e.ctrlKey) {
      generateQRCode();
    }
  });

  async function generateQRCode() {
    const text = textInput.value.trim();
    
    if (!text) {
      alert('Please enter text to generate a QR code');
      return;
    }

    const size = parseInt(sizeInput.value) || 256;
    const errorCorrectionLevel = errorCorrectionSelect.value;

    try {
      await QRCode.toCanvas(qrCanvas, text, {
        width: size,
        errorCorrectionLevel: errorCorrectionLevel,
        margin: 2
      });

      // Update download link
      downloadLink.href = qrCanvas.toDataURL('image/png');
      
      qrResult.classList.remove('hidden');
    } catch (error) {
      console.error('Error generating QR code:', error);
      alert('Error generating QR code: ' + error.message);
    }
  }
});
