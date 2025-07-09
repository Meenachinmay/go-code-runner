-- Insert a sample company
INSERT INTO companies (id, name, email, password_hash, api_key, client_id)
VALUES (1, 'Test Company', 'test@example.com', 'password_hash', 'api_key', 'client_id');

-- Insert a sample problem
INSERT INTO problems (title, description, difficulty)
VALUES ('Two Sum', 'Given two numbers, return their sum.', 'Easy');

-- Insert test cases for the problem
INSERT INTO test_cases (problem_id, input, expected_output, is_hidden)
VALUES 
    (1, '1 2', '3', false),
    (1, '5 7', '12', false),
    (1, '100 200', '300', false),
    (1, '-5 10', '5', true),
    (1, '0 0', '0', true);

-- Insert another sample problem
INSERT INTO problems (title, description, difficulty)
VALUES ('Reverse String', 'Write a function that reverses a string.', 'Easy');

-- Insert test cases for the second problem
INSERT INTO test_cases (problem_id, input, expected_output, is_hidden)
VALUES 
    (2, 'hello', 'olleh', false),
    (2, 'world', 'dlrow', false),
    (2, 'a', 'a', false),
    (2, 'racecar', 'racecar', true),
    (2, '', '', true);
