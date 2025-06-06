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
            flex: 1;
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
        }

        .form-group {
            display: flex;
            flex-direction: column;
            gap: 0.5rem;
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

    <script>
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
