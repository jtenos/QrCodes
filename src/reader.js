import jsQR from 'jsqr';
import heic2any from 'heic2any';

document.addEventListener('DOMContentLoaded', () => {
  const uploadInput = document.getElementById('qr-upload');
  const imagePreview = document.getElementById('image-preview');
  const previewImg = document.getElementById('preview-img');
  const readerResult = document.getElementById('reader-result');
  const rawText = document.getElementById('raw-text');
  const parsedValues = document.getElementById('parsed-values');
  const parsedContent = document.getElementById('parsed-content');
  const copyBtn = document.getElementById('copy-btn');
  const generatorLink = document.getElementById('generator-link');
  const errorMessage = document.getElementById('error-message');

  uploadInput.addEventListener('change', handleFileUpload);
  copyBtn.addEventListener('click', copyToClipboard);

  async function handleFileUpload(event) {
    const file = event.target.files[0];
    if (!file) return;

    // Hide previous results and errors
    readerResult.classList.add('hidden');
    errorMessage.classList.add('hidden');
    imagePreview.classList.add('hidden');

    try {
      let imageBlob = file;
      
      // Check if it's a HEIC file and convert it
      if (file.type === 'image/heic' || file.name.toLowerCase().endsWith('.heic')) {
        try {
          const convertedBlob = await heic2any({
            blob: file,
            toType: 'image/png',
            quality: 1
          });
          imageBlob = Array.isArray(convertedBlob) ? convertedBlob[0] : convertedBlob;
        } catch (heicError) {
          console.error('HEIC conversion error:', heicError);
          showError('Failed to convert HEIC image. Please try a different format.');
          return;
        }
      }

      // Create object URL for preview
      const imageUrl = URL.createObjectURL(imageBlob);
      previewImg.src = imageUrl;
      imagePreview.classList.remove('hidden');

      // Load image and read QR code
      const image = new Image();
      image.onload = () => {
        readQRCode(image);
        URL.revokeObjectURL(imageUrl);
      };
      image.onerror = () => {
        showError('Failed to load the image. Please ensure it is a valid image file.');
        URL.revokeObjectURL(imageUrl);
      };
      image.src = imageUrl;
    } catch (error) {
      console.error('Error processing file:', error);
      showError('Error processing the file: ' + error.message);
    }
  }

  function readQRCode(image) {
    // Create canvas to extract image data
    const canvas = document.createElement('canvas');
    const ctx = canvas.getContext('2d');
    
    canvas.width = image.naturalWidth || image.width;
    canvas.height = image.naturalHeight || image.height;
    
    ctx.drawImage(image, 0, 0);
    
    const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height);
    
    // Use jsQR to decode the QR code
    const code = jsQR(imageData.data, imageData.width, imageData.height, {
      inversionAttempts: 'dontInvert'
    });
    
    if (code) {
      displayResult(code.data);
    } else {
      // Try with inverted colors
      const invertedCode = jsQR(imageData.data, imageData.width, imageData.height, {
        inversionAttempts: 'attemptBoth'
      });
      
      if (invertedCode) {
        displayResult(invertedCode.data);
      } else {
        showError('No QR code found in the image. Please ensure the image contains a clear QR code.');
      }
    }
  }

  function displayResult(data) {
    // Display raw text
    rawText.textContent = data;
    
    // Parse and display structured data if applicable
    const parsed = parseQRData(data);
    if (parsed) {
      parsedContent.innerHTML = '';
      for (const [key, value] of Object.entries(parsed)) {
        const item = document.createElement('div');
        item.className = 'parsed-item';
        item.innerHTML = `
          <span class="parsed-key">${escapeHtml(key)}:</span>
          <span class="parsed-value">${escapeHtml(value)}</span>
        `;
        parsedContent.appendChild(item);
      }
      parsedValues.classList.remove('hidden');
    } else {
      parsedValues.classList.add('hidden');
    }
    
    // Update generator link with the QR code content
    const encodedText = encodeURIComponent(data);
    generatorLink.href = `/?text=${encodedText}`;
    
    readerResult.classList.remove('hidden');
    errorMessage.classList.add('hidden');
  }

  function parseQRData(data) {
    // Try to parse as vCard first (before URL check)
    if (data.startsWith('BEGIN:VCARD')) {
      return parseVCard(data);
    }

    // Try to parse as WiFi configuration (before URL check)
    if (data.startsWith('WIFI:')) {
      return parseWiFi(data);
    }

    // Try to parse as URL (only for http/https URLs)
    try {
      const url = new URL(data);
      // Only treat as URL if it has a standard web protocol
      if (url.protocol === 'http:' || url.protocol === 'https:') {
        const parsed = { Type: 'URL' };
        parsed['Protocol'] = url.protocol.replace(':', '');
        parsed['Host'] = url.host;
        if (url.pathname && url.pathname !== '/') {
          parsed['Path'] = url.pathname;
        }
        if (url.search) {
          parsed['Query'] = url.search;
        }
        if (url.hash) {
          parsed['Hash'] = url.hash;
        }
        return parsed;
      }
    } catch (e) {
      // Not a valid URL
    }

    // Try to parse as phone number
    if (data.startsWith('tel:') || data.startsWith('TEL:')) {
      return {
        Type: 'Phone Number',
        Number: data.substring(4)
      };
    }

    // Try to parse as SMS
    if (data.startsWith('sms:') || data.startsWith('SMS:') || data.startsWith('SMSTO:') || data.startsWith('smsto:')) {
      const smsData = data.replace(/^(sms|SMS|SMSTO|smsto):/, '');
      const parts = smsData.split('?');
      const parsed = {
        Type: 'SMS',
        Number: parts[0]
      };
      if (parts[1]) {
        const bodyMatch = parts[1].match(/body=([^&]*)/i);
        if (bodyMatch) {
          parsed['Message'] = decodeURIComponent(bodyMatch[1]);
        }
      }
      return parsed;
    }

    // Try to parse as email
    if (data.startsWith('mailto:') || data.startsWith('MAILTO:')) {
      const emailData = data.substring(7);
      const parts = emailData.split('?');
      const parsed = {
        Type: 'Email',
        Address: parts[0]
      };
      if (parts[1]) {
        const params = new URLSearchParams(parts[1]);
        if (params.get('subject')) {
          parsed['Subject'] = params.get('subject');
        }
        if (params.get('body')) {
          parsed['Body'] = params.get('body');
        }
      }
      return parsed;
    }

    // Try to parse as geo location
    if (data.startsWith('geo:') || data.startsWith('GEO:')) {
      const geoData = data.substring(4);
      const coords = geoData.split(',');
      if (coords.length >= 2) {
        return {
          Type: 'Location',
          Latitude: coords[0],
          Longitude: coords[1].split('?')[0]
        };
      }
    }

    // Try to parse as JSON
    try {
      const json = JSON.parse(data);
      if (typeof json === 'object' && json !== null) {
        return { Type: 'JSON', ...flattenObject(json) };
      }
    } catch (e) {
      // Not valid JSON
    }

    // Return null if no structured format detected
    return null;
  }

  function parseVCard(data) {
    const parsed = { Type: 'vCard/Contact' };
    const lines = data.split(/\r?\n/);
    
    for (const line of lines) {
      if (line.startsWith('FN:')) {
        parsed['Full Name'] = line.substring(3);
      } else if (line.startsWith('N:')) {
        const nameParts = line.substring(2).split(';');
        if (nameParts[0]) parsed['Last Name'] = nameParts[0];
        if (nameParts[1]) parsed['First Name'] = nameParts[1];
      } else if (line.startsWith('TEL') && line.includes(':')) {
        const tel = line.split(':')[1];
        parsed['Phone'] = tel;
      } else if (line.startsWith('EMAIL') && line.includes(':')) {
        const email = line.split(':')[1];
        parsed['Email'] = email;
      } else if (line.startsWith('ORG:')) {
        parsed['Organization'] = line.substring(4);
      } else if (line.startsWith('TITLE:')) {
        parsed['Title'] = line.substring(6);
      } else if (line.startsWith('URL') && line.includes(':')) {
        const url = line.split(':').slice(1).join(':');
        parsed['Website'] = url;
      } else if (line.startsWith('ADR') && line.includes(':')) {
        const addr = line.split(':')[1].replace(/;/g, ', ').replace(/,\s*,/g, ',').trim();
        if (addr) parsed['Address'] = addr;
      }
    }
    
    return parsed;
  }

  function parseWiFi(data) {
    const parsed = { Type: 'WiFi Network' };
    
    // Remove WIFI: prefix
    const wifiData = data.substring(5);
    
    // Parse WiFi parameters
    const ssidMatch = wifiData.match(/S:([^;]*)/);
    const typeMatch = wifiData.match(/T:([^;]*)/);
    const passwordMatch = wifiData.match(/P:([^;]*)/);
    const hiddenMatch = wifiData.match(/H:([^;]*)/);
    
    if (ssidMatch) parsed['Network Name (SSID)'] = ssidMatch[1];
    if (typeMatch) parsed['Security Type'] = typeMatch[1];
    if (passwordMatch) parsed['Password'] = passwordMatch[1];
    if (hiddenMatch && hiddenMatch[1].toLowerCase() === 'true') {
      parsed['Hidden Network'] = 'Yes';
    }
    
    return parsed;
  }

  function flattenObject(obj, prefix = '') {
    const result = {};
    for (const [key, value] of Object.entries(obj)) {
      const newKey = prefix ? `${prefix}.${key}` : key;
      if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
        Object.assign(result, flattenObject(value, newKey));
      } else {
        result[newKey] = String(value);
      }
    }
    return result;
  }

  function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  }

  function copyToClipboard() {
    const text = rawText.textContent;
    navigator.clipboard.writeText(text).then(() => {
      const originalText = copyBtn.textContent;
      copyBtn.textContent = 'Copied!';
      setTimeout(() => {
        copyBtn.textContent = originalText;
      }, 2000);
    }).catch(err => {
      console.error('Failed to copy:', err);
      alert('Failed to copy to clipboard');
    });
  }

  function showError(message) {
    errorMessage.textContent = message;
    errorMessage.classList.remove('hidden');
    readerResult.classList.add('hidden');
  }
});
