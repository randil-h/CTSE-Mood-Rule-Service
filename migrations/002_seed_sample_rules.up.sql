-- Insert sample rules for testing

-- Rule 1: Happy mood during morning suggests energizing products
INSERT INTO rules (id, name, priority, conditions, actions, weight, version, active)
VALUES (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'Happy Morning Energy',
    100,
    '{"mood": ["happy"], "time_of_day": ["morning"], "logic": "AND"}'::jsonb,
    '{"tags": ["energizing", "fresh", "vibrant"], "categories": ["beverages", "breakfast"], "boost": 0.2}'::jsonb,
    1.5,
    1,
    true
);

-- Rule 2: Sad mood suggests comfort products
INSERT INTO rules (id, name, priority, conditions, actions, weight, version, active)
VALUES (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12',
    'Comfort for Sad Mood',
    90,
    '{"mood": ["sad", "down"], "logic": "OR"}'::jsonb,
    '{"tags": ["comfort", "cozy", "warm"], "categories": ["comfort-food", "wellness"], "price_range": {"min": 0, "max": 100}, "boost": 0.3}'::jsonb,
    1.8,
    1,
    true
);

-- Rule 3: Stressed mood during evening suggests relaxation
INSERT INTO rules (id, name, priority, conditions, actions, weight, version, active)
VALUES (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13',
    'Evening Relaxation',
    85,
    '{"mood": ["stressed", "anxious"], "time_of_day": ["evening", "night"], "logic": "AND"}'::jsonb,
    '{"tags": ["relaxing", "calming", "soothing"], "categories": ["wellness", "aromatherapy"], "boost": 0.25}'::jsonb,
    1.6,
    1,
    true
);

-- Rule 4: Excited mood suggests adventure products
INSERT INTO rules (id, name, priority, conditions, actions, weight, version, active)
VALUES (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14',
    'Adventure for Excitement',
    80,
    '{"mood": ["excited", "energetic"], "logic": "OR"}'::jsonb,
    '{"tags": ["adventure", "outdoor", "active"], "categories": ["sports", "outdoor"], "boost": 0.15}'::jsonb,
    1.4,
    1,
    true
);

-- Rule 5: Rainy weather suggests indoor comfort
INSERT INTO rules (id, name, priority, conditions, actions, weight, version, active)
VALUES (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a15',
    'Rainy Day Comfort',
    75,
    '{"weather": ["rainy", "cloudy"], "logic": "OR"}'::jsonb,
    '{"tags": ["indoor", "cozy", "entertainment"], "categories": ["books", "movies", "games"], "boost": 0.2}'::jsonb,
    1.3,
    1,
    true
);

-- Rule 6: Special occasion premium products
INSERT INTO rules (id, name, priority, conditions, actions, weight, version, active)
VALUES (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a16',
    'Special Occasion Premium',
    95,
    '{"occasion": ["birthday", "anniversary", "celebration"], "logic": "OR"}'::jsonb,
    '{"tags": ["premium", "gift", "special"], "categories": ["gifts", "luxury"], "price_range": {"min": 50, "max": 500}, "boost": 0.4}'::jsonb,
    2.0,
    1,
    true
);

-- Rule 7: Calm mood with sunny weather suggests outdoor activities
INSERT INTO rules (id, name, priority, conditions, actions, weight, version, active)
VALUES (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a17',
    'Sunny Day Outdoors',
    70,
    '{"mood": ["calm", "content"], "weather": ["sunny", "clear"], "logic": "AND"}'::jsonb,
    '{"tags": ["outdoor", "nature", "sunny"], "categories": ["outdoor", "sports", "leisure"], "boost": 0.15}'::jsonb,
    1.2,
    1,
    true
);

-- Rule 8: Late night browsing suggests convenience
INSERT INTO rules (id, name, priority, conditions, actions, weight, version, active)
VALUES (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a18',
    'Late Night Convenience',
    65,
    '{"time_of_day": ["night", "late-night"], "logic": "OR"}'::jsonb,
    '{"tags": ["quick", "convenient", "delivery"], "categories": ["food", "essentials"], "boost": 0.1}'::jsonb,
    1.1,
    1,
    true
);
