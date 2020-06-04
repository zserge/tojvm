public class FieldsAndMethods {
	public int a = 1;
	public static int b = 2;

	public static int add(int x, int y) {
		return x + y;
	}
	public static int mul(int x, int y) {
        return x * y;
    }
	public static int sub(int x, int y) {
		return x - y;
	}
	public void hello() {
		Runtime.log("Hello world");
	}
	public void incrementA() {
		a++;
	}
	public static FieldsAndMethods create() {
		return new FieldsAndMethods();
	}
	public void incrementB() {
		b++;
	}
	public void incrementBoth() {
		incrementA();
		incrementB();
	}
}
