"""
Lightweight OpenAI Performance Test Harness
A simple but effective tool for load testing OpenAI endpoints
"""

import asyncio
import aiohttp
import random
import time
import json
from typing import List, Dict
import hashlib

class UniqueQuestionGenerator:
    """Generate unique questions using various strategies to avoid cache hits"""

    @staticmethod
    def generate_by_strategy(strategy: str, seed: int) -> List[str]:
        """Generate questions based on selected strategy"""

        if strategy == "historical":
            return UniqueQuestionGenerator.historical_questions(seed)
        elif strategy == "mathematical":
            return UniqueQuestionGenerator.mathematical_questions(seed)
        elif strategy == "geographical":
            return UniqueQuestionGenerator.geographical_questions(seed)
        elif strategy == "hypothetical":
            return UniqueQuestionGenerator.hypothetical_questions(seed)
        elif strategy == "technical":
            return UniqueQuestionGenerator.technical_questions(seed)
        else:
            return UniqueQuestionGenerator.mixed_questions(seed)

    @staticmethod
    def historical_questions(seed: int) -> List[str]:
        """Generate questions about random historical years"""
        random.seed(seed)
        year = random.randint(1000, 2023)

        return [
            f"What major events occurred in the year {year}?",
            f"Who were the influential leaders during {year}?",
            f"What was the state of technology in {year}?",
            f"Describe the political climate of {year}.",
            f"What were the major conflicts or peace treaties around {year}?",
            f"How did people communicate in {year}?",
            f"What was daily life like for common people in {year}?",
            f"What scientific discoveries were made around {year}?"
        ]

    @staticmethod
    def mathematical_questions(seed: int) -> List[str]:
        """Generate unique math problems"""
        random.seed(seed)
        a = random.randint(100, 9999)
        b = random.randint(100, 9999)
        c = random.randint(2, 50)

        return [
            f"What is {a} multiplied by {b}?",
            f"If you divide the previous result by {c}, what do you get?",
            f"What are the prime factors of {a}?",
            f"Is {b} a perfect square? If not, what's the nearest one?",
            f"Calculate {a} to the power of {c % 5 + 2}.",
            f"What is the greatest common divisor of {a} and {b}?",
            f"Convert {a} to base-{c % 7 + 2} notation.",
            f"How many ways can you partition {c} into positive integers?"
        ]

    @staticmethod
    def geographical_questions(seed: int) -> List[str]:
        """Generate geography questions with random parameters"""
        random.seed(seed)

        cities = ["Tokyo", "Delhi", "Shanghai", "São Paulo", "Mexico City", "Cairo",
                 "Mumbai", "Beijing", "Dhaka", "Osaka", "Karachi", "Istanbul",
                 "Buenos Aires", "Kolkata", "Lagos", "Manila", "Tianjin", "Rio"]

        countries = ["Brazil", "Russia", "India", "China", "South Africa", "Mexico",
                    "Indonesia", "Turkey", "Saudi Arabia", "Argentina", "Egypt",
                    "Nigeria", "Japan", "Germany", "France", "Italy", "Canada"]

        city = random.choice(cities)
        country = random.choice(countries)
        distance = random.randint(500, 5000)

        return [
            f"What is the population of {city}?",
            f"What are the neighboring countries of {country}?",
            f"What is the main river flowing through or near {city}?",
            f"If you travel {distance}km east from {city}, where might you be?",
            f"What is the climate type in {country}?",
            f"What are the major exports of {country}?",
            f"What language is primarily spoken in {city}?",
            f"What is the time zone of {city}?"
        ]

    @staticmethod
    def hypothetical_questions(seed: int) -> List[str]:
        """Generate creative hypothetical scenarios"""
        random.seed(seed)

        objects = ["cars", "trees", "buildings", "phones", "books", "computers"]
        properties = ["invisible", "magnetic", "telepathic", "indestructible", "sentient"]
        numbers = [10, 50, 100, 1000, 10000]

        obj = random.choice(objects)
        prop = random.choice(properties)
        num = random.choice(numbers)
        percent = random.randint(10, 90)

        return [
            f"What would happen if all {obj} suddenly became {prop}?",
            f"How would society change if {percent}% of people could read minds?",
            f"Design a city that could accommodate {num} million people in 1 square km.",
            f"What if gravity was {percent}% stronger?",
            f"How would communication work if sound didn't exist?",
            f"What if {obj} could only last for 24 hours before disappearing?",
            f"Describe an economy where {obj} are the primary currency.",
            f"What safety measures would we need if everyone could fly?"
        ]

    @staticmethod
    def technical_questions(seed: int) -> List[str]:
        """Generate technical/programming questions"""
        random.seed(seed)

        array_size = random.randint(10, 1000)
        complexity = random.choice(["O(n)", "O(n²)", "O(log n)", "O(n log n)"])
        port = random.randint(3000, 9999)
        hash_input = hashlib.md5(str(seed).encode()).hexdigest()[:8]

        return [
            f"What's the best sorting algorithm for {array_size} nearly-sorted integers?",
            f"How would you design a cache for {array_size} frequently accessed items?",
            f"Explain the trade-offs of using a hash table with {array_size} buckets.",
            f"What happens when you try to connect to port {port}?",
            f"Design a URL shortener that generates codes like '{hash_input}'.",
            f"How would you find duplicates in an array of {array_size} elements?",
            f"What's the space complexity of storing {array_size} items in a binary tree?",
            f"How would you implement rate limiting for {array_size} requests per second?"
        ]

    @staticmethod
    def mixed_questions(seed: int) -> List[str]:
        """Mix different types of questions"""
        random.seed(seed)
        questions = []

        # Take 2 questions from each category
        for strategy in ["historical", "mathematical", "geographical", "hypothetical", "technical"]:
            strategy_questions = UniqueQuestionGenerator.generate_by_strategy(strategy, seed + hash(strategy))
            questions.extend(random.sample(strategy_questions, 2))

        return questions[:8]  # Return 8 questions


class SimpleTestHarness:
    """Simplified test harness for OpenAI API testing"""

    def __init__(self, api_key: str, model: str = "gpt-3.5-turbo"):
        self.api_key = api_key
        self.model = model
        self.results = []

    async def test_conversation(self, session: aiohttp.ClientSession,
                               client_id: int, questions: List[str]) -> Dict:
        """Test a single conversation flow"""

        url = "https://api.openai.com/v1/chat/completions"
        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json"
        }

        conversation_results = []
        messages = []

        for i, question in enumerate(questions):
            messages.append({"role": "user", "content": question})

            payload = {
                "model": self.model,
                "messages": messages,
                "temperature": 0.7,
                "max_tokens": 150
            }

            start_time = time.perf_counter()

            try:
                async with session.post(url, headers=headers, json=payload) as response:
                    latency = time.perf_counter() - start_time

                    if response.status == 200:
                        data = await response.json()

                        # Add assistant response to conversation
                        assistant_msg = data['choices'][0]['message']['content']
                        messages.append({"role": "assistant", "content": assistant_msg})

                        conversation_results.append({
                            "question_num": i + 1,
                            "latency": latency,
                            "tokens": data.get('usage', {}).get('total_tokens', 0),
                            "success": True
                        })
                    else:
                        conversation_results.append({
                            "question_num": i + 1,
                            "latency": latency,
                            "success": False,
                            "error": f"Status {response.status}"
                        })
                        break  # Stop conversation on error

            except Exception as e:
                conversation_results.append({
                    "question_num": i + 1,
                    "latency": time.perf_counter() - start_time,
                    "success": False,
                    "error": str(e)
                })
                break

            # Small delay between questions
            await asyncio.sleep(0.1)

        return {
            "client_id": client_id,
            "results": conversation_results
        }

    async def run_load_test(self, num_clients: int = 100,
                           max_concurrent: int = 20,
                           question_strategy: str = "mixed") -> Dict:
        """Run the load test with specified parameters"""

        print(f"\nStarting load test:")
        print(f"  - Clients: {num_clients}")
        print(f"  - Max concurrent: {max_concurrent}")
        print(f"  - Strategy: {question_strategy}")
        print(f"  - Model: {self.model}")

        semaphore = asyncio.Semaphore(max_concurrent)

        async def run_client(session, client_id):
            async with semaphore:
                # Generate unique questions for this client
                seed = client_id * 1337  # Use client_id to ensure uniqueness
                questions = UniqueQuestionGenerator.generate_by_strategy(
                    question_strategy, seed
                )[:5]  # Use 5 questions per conversation

                return await self.test_conversation(session, client_id, questions)

        start_time = time.time()

        async with aiohttp.ClientSession() as session:
            tasks = [run_client(session, i) for i in range(num_clients)]
            self.results = await asyncio.gather(*tasks)

        duration = time.time() - start_time

        return self.analyze_results(duration)

    def analyze_results(self, duration: float) -> Dict:
        """Analyze and summarize test results"""

        all_requests = []
        for client_result in self.results:
            all_requests.extend(client_result['results'])

        successful = [r for r in all_requests if r.get('success', False)]
        failed = [r for r in all_requests if not r.get('success', False)]

        if successful:
            latencies = [r['latency'] for r in successful]
            latencies.sort()

            stats = {
                "duration_seconds": duration,
                "total_clients": len(self.results),
                "total_requests": len(all_requests),
                "successful_requests": len(successful),
                "failed_requests": len(failed),
                "success_rate": f"{len(successful)/len(all_requests)*100:.1f}%",
                "throughput_rps": len(all_requests) / duration,
                "latency_stats": {
                    "min": f"{min(latencies):.3f}s",
                    "max": f"{max(latencies):.3f}s",
                    "avg": f"{sum(latencies)/len(latencies):.3f}s",
                    "p50": f"{latencies[len(latencies)//2]:.3f}s",
                    "p95": f"{latencies[int(len(latencies)*0.95)]:.3f}s",
                    "p99": f"{latencies[int(len(latencies)*0.99)]:.3f}s" if len(latencies) > 100 else "N/A"
                }
            }

            if any('tokens' in r for r in successful):
                total_tokens = sum(r.get('tokens', 0) for r in successful)
                stats['token_stats'] = {
                    "total_tokens": total_tokens,
                    "avg_tokens_per_request": total_tokens // len(successful)
                }
        else:
            stats = {
                "error": "No successful requests",
                "duration_seconds": duration,
                "failed_requests": len(failed)
            }

        return stats


# Quick example script
async def example_run():
    """Example of how to use the test harness"""

    import os

    # Get API key from environment variable
    api_key = os.environ.get("OPENAI_API_KEY")
    if not api_key:
        print("Please set OPENAI_API_KEY environment variable")
        return

    # Create test harness
    harness = SimpleTestHarness(api_key, model="gpt-3.5-turbo")

    # Run a small test
    results = await harness.run_load_test(
        num_clients=10,  # Start small
        max_concurrent=5,
        question_strategy="mixed"  # or "historical", "mathematical", etc.
    )

    # Print results
    print("\n" + "="*50)
    print("TEST RESULTS")
    print("="*50)
    print(json.dumps(results, indent=2))

    # Save to file
    with open("test_results.json", "w") as f:
        json.dump(results, f, indent=2)
    print("\nResults saved to test_results.json")


if __name__ == "__main__":
    # Test the question generator
    if "--test-questions" in str(__import__("sys").argv):
        print("Sample questions from each strategy:\n")
        for strategy in ["historical", "mathematical", "geographical", "hypothetical", "technical"]:
            print(f"\n{strategy.upper()} QUESTIONS:")
            questions = UniqueQuestionGenerator.generate_by_strategy(strategy, 42)
            for i, q in enumerate(questions[:3], 1):
                print(f"  {i}. {q}")
    else:
        # Run the example
        asyncio.run(example_run())
