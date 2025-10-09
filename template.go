package main

const portal = `<!DOCTYPE html>
<html lang='en'>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Online Dictionary</title>
    <style>
        * {
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            line-height: 1.6;
            color: #333;
            background-color: #f5f5f5;
            min-height: 100vh;
            display: flex;
            flex-direction: column;
        }

        .container {
            max-width: 600px;
            margin: 2rem auto;
            padding: 2rem;
            background: white;
            border-radius: 10px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            min-height: 400px;
            display: flex;
            flex-direction: column;
        }

        h1 {
            text-align: center;
            color: #2196F3;
            margin-bottom: 2rem;
            font-size: 2rem;
        }

        .search-form {
            display: flex;
            flex-direction: column;
            gap: 1rem;
            flex: 1;
            justify-content: flex-start;
            margin-bottom: 2rem;
        }

        .form-group {
            display: flex;
            flex-direction: column;
            gap: 0.5rem;
            position: relative;
            margin-bottom: 1rem;
        }

        .form-group:first-child {
            margin-bottom: 1.5rem;
        }

        label {
            font-weight: 500;
            color: #666;
        }

        input[type="text"] {
            padding: 0.8rem;
            border: 2px solid #e0e0e0;
            border-radius: 6px;
            font-size: 1rem;
            transition: border-color 0.3s ease;
        }

        input[type="text"]:focus {
            outline: none;
            border-color: #2196F3;
        }

        input[type="submit"] {
            background: #2196F3;
            color: white;
            border: none;
            padding: 1rem;
            border-radius: 6px;
            font-size: 1rem;
            cursor: pointer;
            transition: background-color 0.3s ease;
            margin-top: 1rem;
            min-height: 48px;
            width: 100%;
        }

        input[type="submit"]:hover {
            background: #1976D2;
        }

        footer {
            text-align: center;
            padding: 1.5rem;
            background-color: #2196F3;
            color: white;
            margin-top: auto;
        }

        footer a {
            color: white;
            text-decoration: none;
            border-bottom: 1px dotted white;
        }

        footer a:hover {
            border-bottom: 1px solid white;
        }

        @media (max-width: 640px) {
            .container {
                margin: 1rem;
                padding: 1rem;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Online Dictionary</h1>
        <form class="search-form" action="/dict" method="get">
            <div class="form-group">
                <label for="query">Query:</label>
                <input type="text" id="query" name="query" placeholder="Enter a word..." required autocomplete="on"/>
            </div>
            
            <div class="form-group">
                <label for="engine">Engine:</label>
                <input type="text" id="engine" name="engine" value="mdx" placeholder="mdx"/>
            </div>

            <input type="hidden" name="format" value="html" />
            <input type="hidden" name="record" value="1" />
            <input type="submit" value="Search"/>
        </form>
    </div>

    <footer>
        <p>This is an open-source project</p>
        <p>Author: ChaosNyaruko</p>
        <p><a href="https://github.com/ChaosNyaruko/ondict">GitHub Repository</a></p>
    </footer>

	<style>
    .suggestions {
        list-style-type: none;
        margin: 0;
        padding: 0;
        border: 1px solid #e0e0e0;
        border-top: none;
        border-radius: 0 0 6px 6px;
        background-color: white;
        max-height: 200px;
        overflow-y: auto;
        position: absolute;
        top: 100%;
        left: 0;
        right: 0;
        z-index: 1000;
        box-shadow: 0 2px 8px rgba(0,0,0,0.1);
    }

    .suggestions li {
        padding: 0.8rem;
        cursor: pointer;
        transition: background-color 0.3s ease;
    }

    .suggestions li:hover, .suggestions li.active {
        background-color: #f5f5f5;
    }

    .form-group {
        position: relative;
    }
</style>

<script>
        // Autocomplete functionality
const queryInput = document.getElementById('query');
const suggestionsList = document.createElement('ul');
suggestionsList.className = 'suggestions';
const formGroup = queryInput.parentNode;
formGroup.appendChild(suggestionsList);
let activeIndex = -1;

// Function to update active suggestion highlighting
function updateActiveSuggestion(suggestions) {
    suggestions.forEach((item, index) => {
        item.classList.toggle('active', index === activeIndex);
    });
    if (activeIndex >= 0 && activeIndex < suggestions.length) {
        suggestions[activeIndex].scrollIntoView({ block: 'nearest' });
    }
}

// Debounce function
function debounce(func, delay) {
    let timeoutId;
    return function(...args) {
        clearTimeout(timeoutId);
        timeoutId = setTimeout(() => func.apply(this, args), delay);
    };
}

function updateActiveSuggestion(suggestions) {
  suggestions.forEach((item, index) => {
    item.classList.toggle('active', index === activeIndex);
  });
  if (activeIndex >= 0 && activeIndex < suggestions.length) {
    suggestions[activeIndex].scrollIntoView({ block: 'nearest' });
  }
}

// Handle input events for autocomplete
queryInput.addEventListener('input', debounce(async (e) => {
    const prefix = e.target.value.trim();
    suggestionsList.innerHTML = '';

    if (prefix.length < 2) {
        return;
    }

    try {
        const response = await ` + "fetch(`/complete?prefix=${encodeURIComponent(prefix)}`)\n" + `;
        if (!response.ok) {
	` +
	" throw new Error(`HTTP error! status: ${response.status}`);" + `
        }
        const suggestions = await response.json();

        if (suggestions.length === 0) {
            return;
        }

        suggestions.forEach(word => {
            const li = document.createElement('li');
            li.textContent = word;
            li.addEventListener('click', () => {
                queryInput.value = word;
                suggestionsList.innerHTML = '';
            });
            suggestionsList.appendChild(li);
        });
    } catch (err) {
        console.error('Error fetching autocomplete suggestions:', err);
    }
}, 800));

// Handle keydown events for navigation
queryInput.addEventListener('keydown', (e) => {
  const suggestions = suggestionsList.querySelectorAll('li');
  if (e.key === 'ArrowDown') {
    e.preventDefault();
    activeIndex = Math.min(activeIndex + 1, suggestions.length - 1);
    updateActiveSuggestion(suggestions);
  } else if (e.key === 'ArrowUp') {
    e.preventDefault();
    activeIndex = Math.max(activeIndex - 1, 0);
    updateActiveSuggestion(suggestions);
  } else if (e.key === 'Enter') {
    e.preventDefault();
    if (activeIndex >= 0 && activeIndex < suggestions.length) {
      queryInput.value = suggestions[activeIndex].textContent;
      suggestionsList.innerHTML = '';
      activeIndex = -1;
    }
  }
});

// Handle keydown events for navigation and form submission
queryInput.addEventListener('keydown', (e) => {
    const suggestions = suggestionsList.querySelectorAll('li');
    
    if (e.key === 'ArrowDown') {
        e.preventDefault();
        activeIndex = Math.min(activeIndex + 1, suggestions.length - 1);
        updateActiveSuggestion(suggestions);
    } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        activeIndex = Math.max(activeIndex - 1, 0);
        updateActiveSuggestion(suggestions);
    } else if (e.key === 'Enter') {
        e.preventDefault();
        if (activeIndex >= 0 && activeIndex < suggestions.length) {
            queryInput.value = suggestions[activeIndex].textContent;
            suggestionsList.innerHTML = '';
            activeIndex = -1;
        } else {
            // Submit form if no suggestion is selected
            document.querySelector('.search-form').submit();
        }
    }
});

// Clear suggestions when clicking outside
document.addEventListener('click', (e) => {
    if (!formGroup.contains(e.target)) {
        suggestionsList.innerHTML = '';
        activeIndex = -1;
    }
});
        // Focus the query input when page loads
        document.addEventListener('DOMContentLoaded', () => {
            document.getElementById('query').focus();
        });

        // Simple form validation
        document.querySelector('.search-form').addEventListener('submit', (e) => {
            const query = document.getElementById('query').value.trim();
            if (!query) {
                e.preventDefault();
                alert('Please enter a word to search');
            }
        });
    </script>
</body>
</html>
`

const reviewPage = `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Submit Page</title>
  <style>
    .form-group {
      display: flex;
      align-items: center;
      margin-bottom: 10px;
    }

    .form-group label {
      width: 100px;
      font-weight: bold;
    }

    .form-group input {
      flex: 1;
      padding: 5px;
    }

    button {
      margin-top: 10px;
      padding: 6px 12px;
    }
    body {
		font-family: Arial, sans-serif;
		margin: 0;
		padding: 0;
	}

	.container {
		width: 90%;
		max-width: 1200px;
		margin: auto;
	}

	@media (max-width: 600px) {
		.container {
			padding: 10px;
		}
	}

	.nav {
		display: flex;
		justify-content: space-between;
	}

	.nav-item {
		flex: 1;
		text-align: center;
	}
</style>
</head>
<body>
  <h1>Submit Data</h1>
  <form id="dataForm" action="/review" method="GET">
    <div class="form-group">
      <label for="count">Count:</label>
      <input type="number" id="count" name="count" value="1" min="1" required>
    </div>

    <div class="form-group">
      <label for="daysAgo">X Days Ago:</label>
      <input type="number" id="daysAgo" name="days_ago" value="7" required>
    </div>

    <button type="submit">Submit</button>
  </form>
</body>
</html>
`
const login = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Login</title>
</head>
<body>
    <h1>Login</h1>
    {{if .error}}<p style="color:red;">{{.error}}</p>{{end}}
    <form method="post" action="/login">
        Username: <input type="text" name="username"><br>
        Password: <input type="password" name="password"><br>
        <input type="submit" value="Login">
    </form>
</body>
</html>
`
